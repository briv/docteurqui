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

type browserControl struct {
	devToolsProtocolConn *rpcc.Conn
	oneTargetClient      *cdp.Client
}

func (bc *browserControl) cleanup() {
	if bc.devToolsProtocolConn != nil {
		// Leaving connections open will leak memory otherwise.
		bc.devToolsProtocolConn.Close()
	}
}

type Control struct {
	devToolsConnUrl     string
	devToolsConnTimeout time.Duration
	browserControl      *browserControl
	Template            *template.Template
	mutex               csync.Mutex
}

func (pdfGen *Control) Init(templateFilePath string, url string, connectionTimeout time.Duration) error {
	// Initialize template.
	b, err := ioutil.ReadFile(templateFilePath)
	if err != nil {
		return err
	}

	t, err := template.New("rempla-contract").Parse(string(b))
	if err != nil {
		return err
	}
	pdfGen.Template = t

	pdfGen.devToolsConnUrl = url
	pdfGen.devToolsConnTimeout = connectionTimeout

	ctx, cancel := context.WithTimeout(context.Background(), pdfGen.devToolsConnTimeout)
	defer cancel()
	// Try to connect right-away to the remote-controlled browser as an optimization,
	// but ignore any error.
	pdfGen.setupBrowserControlIfNeeded(ctx)

	return nil
}

func (pdfGen *Control) setupBrowserControlIfNeeded(ctx context.Context) error {
	if pdfGen.browserControl != nil {
		return nil
	}

	// initialize connection to browser (PDF renderer)

	// Use the DevTools HTTP/JSON API to manage targets (e.g. pages, webworkers).
	devt := devtool.New(pdfGen.devToolsConnUrl)
	pt, err := devt.Get(ctx, devtool.Page)
	if err != nil {
		// TODO: improve manager for concurrent access to the single chromium frame/target for pdf generation. This Create() call is where we create a new page. Instead of having just one, we could use a pool perhaps.
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

	pdfGen.browserControl = &browserControl{
		devToolsProtocolConn: conn,
		oneTargetClient:      cdp.NewClient(conn),
	}
	return nil
}

func (pdfGen *Control) Shutdown() {
	if pdfGen.browserControl != nil {
		pdfGen.browserControl.cleanup()
	}
}

func (pdfGen *Control) GeneratePdf(ctx context.Context, url string) ([]byte, error) {
	if err := pdfGen.mutex.CLock(ctx); err != nil {
		// Failed to lock.
		return nil, err
	}
	defer pdfGen.mutex.Unlock()

	setupCtx, cancel := context.WithTimeout(ctx, pdfGen.devToolsConnTimeout)
	defer cancel()
	if err := pdfGen.setupBrowserControlIfNeeded(setupCtx); err != nil {
		return nil, err
	}

	data, err := pdfFromUrl(ctx, url, pdfGen.browserControl.oneTargetClient)
	if err != nil {
		pdfGen.browserControl.cleanup()
		// Reset our browser connection if something went wrong.
		pdfGen.browserControl = nil
		return nil, err
	}
	return data, nil
}

func pdfFromUrl(ctx context.Context, url string, c *cdp.Client) ([]byte, error) {
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
	_, err = c.Page.Navigate(ctx, navArgs)
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
