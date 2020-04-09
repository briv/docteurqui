package pdfgen

import (
	"context"
	"fmt"
	"html/template"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/mafredri/cdp"
	"github.com/mafredri/cdp/devtool"
	"github.com/mafredri/cdp/protocol/network"
	"github.com/mafredri/cdp/protocol/page"
	"github.com/mafredri/cdp/rpcc"
	"github.com/sasha-s/go-csync"
)

// TODO: improve manager for concurrent access to the single chromium frame/target for pdf generation
type Control struct {
	devToolsConnUrl      string
	devToolsConnTimeout  time.Duration
	devToolsProtocolConn *rpcc.Conn
	oneTargetClient      *cdp.Client
	Template             *template.Template
	mutex                csync.Mutex
}

func (pdfGen *Control) Init(url string, connectionTimeout time.Duration) error {
	// TODO: finish real template generation

	// initialize templates
	// TODO: this should not be hard-coded.
	b, err := ioutil.ReadFile("/Users/blaiserivet/Documents/Blaise/dev/autoContratRempla/src/contract-templates/dist/index.html")
	if err != nil {
		return err
	}

	t, err := template.New("rempla-étudiant").Parse(string(b))
	if err != nil {
		return err
	}
	pdfGen.Template = t

	pdfGen.devToolsConnUrl = url
	pdfGen.devToolsConnTimeout = connectionTimeout

	pdfGen.connectIfNeeded()

	return nil
}

func (pdfGen *Control) connectIfNeeded() error {
	if pdfGen.devToolsProtocolConn != nil {
		return nil
	}

	// initialize connection to browser (PDF renderer)
	ctx, _ := context.WithTimeout(context.Background(), pdfGen.devToolsConnTimeout)

	// Use the DevTools HTTP/JSON API to manage targets (e.g. pages, webworkers).
	devt := devtool.New(pdfGen.devToolsConnUrl)
	pt, err := devt.Get(ctx, devtool.Page)
	if err != nil {
		pt, err = devt.Create(ctx)
		if err != nil {
			return err
		}
	}

	// Initiate a new RPC connection to the Chrome DevTools Protocol target.
	conn, err := rpcc.DialContext(ctx, pt.WebSocketDebuggerURL)
	if err != nil {
		return err
	}

	pdfGen.devToolsProtocolConn = conn
	pdfGen.oneTargetClient = cdp.NewClient(conn)

	return nil
}

func (pdfGen *Control) Shutdown() {
	if pdfGen.devToolsProtocolConn != nil {
		// Leaving connections open will leak memory.
		pdfGen.devToolsProtocolConn.Close()
	}
}

func (pdfGen *Control) GeneratePdf(ctx context.Context, url string) ([]byte, error) {
	err := pdfGen.mutex.CLock(ctx)
	if err != nil {
		// Failed to lock.
		return nil, err
	}
	defer pdfGen.mutex.Unlock()

	err = pdfGen.connectIfNeeded()
	if err != nil {
		return nil, err
	}

	c := pdfGen.oneTargetClient

	// Open a DOMContentEventFired client to buffer this event.
	loadEventClient, err := c.Page.LoadEventFired(ctx)
	if err != nil {
		return nil, err
	}
	defer loadEventClient.Close()

	networkResponseReceivedClient, err := c.Network.ResponseReceived(ctx)
	if err != nil {
		return nil, err
	}
	defer networkResponseReceivedClient.Close()

	// Enable events on the Page domain, it's often preferrable to create
	// event clients before enabling events so that we don't miss any.
	if err = c.Page.Enable(ctx); err != nil {
		return nil, err
	}
	if err = c.Network.Enable(ctx, network.NewEnableArgs()); err != nil {
		return nil, err
	}

	// Create the Navigate arguments with the optional Referrer field set.
	navArgs := page.NewNavigateArgs(url)
	// SetReferrer("https://duckduckgo.com")
	nav, err := c.Page.Navigate(ctx, navArgs)
	if err != nil {
		return nil, err
	}

	var responseRecievedReply *network.ResponseReceivedReply
	if responseRecievedReply, err = networkResponseReceivedClient.Recv(); err != nil {
		return nil, err
	}

	httpStatus := responseRecievedReply.Response.Status
	// TODO: 304 is only ok for live debug DEV mode
	if !(httpStatus == http.StatusOK || httpStatus == http.StatusNotModified) {
		return nil, fmt.Errorf("unexpected HTTP status for PDF generation: %d", httpStatus)
	}

	// Wait until we have a DOMContentEventFired event.
	if _, err = loadEventClient.Recv(); err != nil {
		return nil, err
	}

	fmt.Printf("Page loaded with frame ID: %s\n", nav.FrameID)

	// footerTemplate := `<span class=pageNumber></span><span class=totalPages></span>`
	pdfArgs := page.NewPrintToPDFArgs().
		SetPreferCSSPageSize(true).
		SetPrintBackground(true).
		SetDisplayHeaderFooter(false)
		// SetHeaderTemplate("").
		// SetFooterTemplate(footerTemplate)
	pdf, err := c.Page.PrintToPDF(ctx, pdfArgs)
	if err != nil {
		return nil, err
	}

	return pdf.Data, nil
}
