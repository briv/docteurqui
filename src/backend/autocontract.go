package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strconv"
	"strings"
	"time"

	"autocontract/internal/csp"
	"autocontract/internal/datamap"
	"autocontract/internal/doctorsearch"
	"autocontract/internal/form"
	"autocontract/internal/httperror"
	"autocontract/internal/pdfgen"
)

const (
	PublicFacingWebsitePort = "18080"

	TimeLayout              = "2006-01-02"
	ParseFormMaxMemoryBytes = 500 * 1024
)

const (
	PdfGeneratorInitializationTimeout = 3 * time.Second
	PdfGeneratorBrowserDevToolsUrl    = "http://localhost:9222"
	PdfGenerationTimeout              = 10 * time.Second
	PurePdfGenerationTimeoutDiff      = 1 * time.Second

	DoctorSearchNGramSize            = 3
	DoctorSearchQueryTimeout         = 5 * time.Second
	DoctorSearchMaxNumberResults     = 5
	MaxDoctorSearchQueryLength       = 40
	MaxDoctorSearchConcurrentQueries = 100

	InternalHttpServerPdfTemplatePath                = "/pdf"
	InternalHttpServerPdfTemplatePort                = "18081"
	InternalHttpServerPdfTemplateRequestUserQueryKey = "userDataId"

	ContextUserDataMapKey = iota
	ContextDoctorSearchKey
	ContextPdfGenControlKey
	ContextTimeZoneLocationKey
)

var (
	SharedUserData       = datamap.NewDataMap()
	SharedDoctorSearcher doctorsearch.DoctorSearcher
	SharedPdfGenControl  = &pdfgen.Control{}
)

func sharedUserDataFromContext(ctx context.Context) datamap.DataMap {
	return ctx.Value(ContextUserDataMapKey).(datamap.DataMap)
}

func sharedDoctorSearcherFromContext(ctx context.Context) doctorsearch.DoctorSearcher {
	return ctx.Value(ContextDoctorSearchKey).(doctorsearch.DoctorSearcher)
}

func pdfGenControlFromContext(ctx context.Context) *pdfgen.Control {
	return ctx.Value(ContextPdfGenControlKey).(*pdfgen.Control)
}

func doctorQueryIndexFromContext(ctx context.Context) *pdfgen.Control {
	return ctx.Value(ContextPdfGenControlKey).(*pdfgen.Control)
}

func timeZoneLocationFromContext(ctx context.Context) *time.Location {
	return ctx.Value(ContextTimeZoneLocationKey).(*time.Location)
}

func withContext(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		var ctx context.Context
		ctx = context.WithValue(req.Context(), ContextUserDataMapKey, SharedUserData)
		ctx = context.WithValue(ctx, ContextDoctorSearchKey, SharedDoctorSearcher)
		ctx = context.WithValue(ctx, ContextPdfGenControlKey, SharedPdfGenControl)
		h(w, req.WithContext(ctx))
	}
}

func withTimeZoneLocation(location *time.Location, h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		var ctx context.Context
		ctx = context.WithValue(req.Context(), ContextTimeZoneLocationKey, location)
		h(w, req.WithContext(ctx))
	}
}

func forMethod(method string, h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		if req.Method != method {
			w.Header().Set("Allow", method)
			http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
			return
		}

		h(w, req)
	}
}

func genContractHandler(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

	ctx, cancel := context.WithTimeout(r.Context(), PdfGenerationTimeout)
	defer cancel()

	timeLocation := timeZoneLocationFromContext(ctx)
	err := r.ParseMultipartForm(ParseFormMaxMemoryBytes)
	if err != nil {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	safeUserData, err := form.Process(r, form.FormProcessingManner{
		TimeLocation: timeLocation,
		TimeLayout:   TimeLayout,
	})
	if err != nil {
		log.Printf("form processing error %v", err)
		httperror.RichError(w, r, err)
		return
	}

	// stuff user data in shared map, addressed by uuid
	sharedUserData := sharedUserDataFromContext(r.Context())
	userDataKey, err := sharedUserData.Set(safeUserData)
	if err != nil {
		log.Println(err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	defer sharedUserData.Clear(userDataKey)

	q := url.Values{}
	q.Set(InternalHttpServerPdfTemplateRequestUserQueryKey, userDataKey)
	pdfUrl := &url.URL{
		Scheme:   "http",
		Host:     fmt.Sprintf("host.docker.internal:%s", InternalHttpServerPdfTemplatePort),
		Path:     InternalHttpServerPdfTemplatePath,
		RawQuery: q.Encode(),
	}

	// ************
	// TODO: debug mode for connecting directly to dev mode of HTML behind the PDFs
	// pdfUrl = &url.URL{
	// 	Scheme: "http",
	// 	Host:   fmt.Sprintf("host.docker.internal:%s", "1234"),
	// 	Path:   "/",
	// }
	// ************
	log.Println(pdfUrl.String())
	pdfGenerator := pdfGenControlFromContext(r.Context())

	// try to leave some time for writing the HTTP response (error) if we aren't able to generate the
	// pdf in time.
	currentDeadline, _ := ctx.Deadline()
	purePdfGenerationContext, cancelFunc := context.WithDeadline(ctx, currentDeadline.Add(-PurePdfGenerationTimeoutDiff))
	defer cancelFunc()

	pdfData, err := pdfGenerator.GeneratePdf(purePdfGenerationContext, pdfUrl.String())
	if err != nil {
		log.Println(err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Length", strconv.Itoa(len(pdfData)))
	_, err = w.Write(pdfData)
	if err != nil {
		// TODO: just log error, not much else we can do right ?
		log.Println(err)
	}

	elapsed := time.Since(start)
	log.Printf("Generating PDF from user data took %s", elapsed)
}

func doctorSearchHandler(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), DoctorSearchQueryTimeout)
	defer cancel()

	sharedDoctorSearcher := sharedDoctorSearcherFromContext(ctx)

	userQuery := strings.TrimSpace(r.URL.Query().Get("query"))
	potentialDoctorMatches, err := sharedDoctorSearcher.Query(ctx, userQuery, DoctorSearchMaxNumberResults)

	if err != nil {
		if errors.Is(err, doctorsearch.TemporarilyUnavailable) {
			http.Error(w, http.StatusText(http.StatusServiceUnavailable), http.StatusServiceUnavailable)
		} else if errors.Is(err, doctorsearch.InvalidUserQuery) {
			log.Printf("error: %v\n", err)
			http.Error(w, http.StatusText(http.StatusUnprocessableEntity), http.StatusUnprocessableEntity)
		} else {
			// TODO: log ?
			log.Printf("error: %v\n", err)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	b, err := json.Marshal(
		struct {
			Matches []doctorsearch.DoctorRecord `json:"matches"`
		}{
			potentialDoctorMatches,
		},
	)
	if err != nil {
		// TODO: log ?
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	w.Write(b)
}

func pdfTemplateHandler(w http.ResponseWriter, r *http.Request) {
	// As an extra paranoid step, use a CSP header for internal web-server used for PDF generation.
	// Go's html/template package is used for escaping user content so injections shouldn't be an issue,
	// but defense in depth can't hurt.
	w.Header().Set(csp.CSPHeader, csp.PdfCSPHeader)

	userDataKey := r.URL.Query().Get(InternalHttpServerPdfTemplateRequestUserQueryKey)

	sharedUserData := sharedUserDataFromContext(r.Context())
	userData, err := sharedUserData.Get(userDataKey)
	if err != nil {
		log.Println(err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	pdfGenerator := pdfGenControlFromContext(r.Context())

	err = pdfGenerator.Template.Execute(w, userData)
	if err != nil {
		log.Println(err)
	}
}

// This go program hosts 3 actors:
// - The public-facing "website" & API HTTP server, which serves web assets (HTML, CSS etc..) and
//   also responds to requests for PDF generation.
//
// - The PDF generating service, which interacts with a headless Chrome process.
// It also uses an HTTP server, but bound only to localhost, for the headless web browser to connect to.
//
// - A statistics/analytics service which records and also serves a public facing route to see them.
func main() {
	publicFacingWebsitePort := flag.String("p", PublicFacingWebsitePort, "port to serve on")
	publicFacingWebsitePathRoot := flag.String("d", "../../dist/", "the directory containing files to host over HTTP")
	devWebsiteProxyPort := flag.String("dev", "", "a port to reverse-proxy the user-facing web HTTP requests (useful for developping front-end)")
	flag.Parse()

	err := SharedPdfGenControl.Init(PdfGeneratorBrowserDevToolsUrl, PdfGeneratorInitializationTimeout)
	if err != nil {
		log.Fatal(err)
	}
	defer SharedPdfGenControl.Shutdown()

	// load time-zone for Paris
	parisLocation, err := time.LoadLocation("Europe/Paris")
	if err != nil {
		log.Fatal(err)
	}

	// Setup doctor search structure
	// TODO: use flag for value
	SharedDoctorSearcher = doctorsearch.New("/Users/blaiserivet/Documents/Blaise/dev/autoContratRempla/src/test-search/PS_LibreAcces_202003041402/", DoctorSearchNGramSize, MaxDoctorSearchQueryLength, MaxDoctorSearchConcurrentQueries)

	// Internal HTTP server for use with headless Web browser instance to convert web pages to PDF.
	errChan := make(chan error)
	go func() {
		pdfServeMux := http.NewServeMux()
		pdfServeMux.HandleFunc(InternalHttpServerPdfTemplatePath, withContext(forMethod(http.MethodGet, pdfTemplateHandler)))

		pdfServer := &http.Server{
			Addr:              fmt.Sprintf("localhost:%s", InternalHttpServerPdfTemplatePort),
			Handler:           pdfServeMux,
			ReadHeaderTimeout: 10 * time.Second,
			ReadTimeout:       10 * time.Second,
			WriteTimeout:      PdfGenerationTimeout,
			MaxHeaderBytes:    1 << 20,
		}
		err := pdfServer.ListenAndServe()
		if err != nil {
			errChan <- err
		}
	}()

	// Public-facing HTTP server.
	go func() {
		publicServeMux := http.NewServeMux()

		var rootHandler http.Handler
		if *devWebsiteProxyPort == "" {
			rootHandler = http.FileServer(http.Dir(*publicFacingWebsitePathRoot))
		} else {
			urlToProxyTo, err := url.Parse(fmt.Sprintf("http://localhost:%s/", *devWebsiteProxyPort))
			if err != nil {
				log.Fatal(err)
			}
			rootHandler = httputil.NewSingleHostReverseProxy(urlToProxyTo)
		}
		publicServeMux.Handle("/", csp.WithSecurityHeaders(rootHandler))

		publicServeMux.HandleFunc("/b/generate-contract", withContext(
			withTimeZoneLocation(parisLocation, forMethod(http.MethodPost, genContractHandler))))

		publicServeMux.HandleFunc("/b/search-doctor", withContext(forMethod(http.MethodGet, doctorSearchHandler)))

		s := &http.Server{
			Addr:              fmt.Sprintf(":%s", *publicFacingWebsitePort),
			Handler:           publicServeMux,
			ReadHeaderTimeout: 10 * time.Second,
			ReadTimeout:       10 * time.Second,
			WriteTimeout:      PdfGenerationTimeout,
			MaxHeaderBytes:    1 << 20,
		}
		err := s.ListenAndServe()
		if err != nil {
			errChan <- err
		}
	}()

	fmt.Printf("Auto-contract service starting: http://localhost:%s\n", *publicFacingWebsitePort)
	err = <-errChan
	log.Fatal(err)
}
