package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"autocontract/internal/csp"
	"autocontract/internal/datamap"
	"autocontract/internal/doctorsearch"
	"autocontract/internal/form"
	"autocontract/internal/httperror"
	"autocontract/internal/pdfgen"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
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
	ContextInternalTemplateWebHostKey
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

func internalTemplateWebHostnameFromContext(ctx context.Context) string {
	return ctx.Value(ContextInternalTemplateWebHostKey).(string)
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

func withInternalTemplateWebHostname(host string, h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		var ctx context.Context
		ctx = context.WithValue(req.Context(), ContextInternalTemplateWebHostKey, host)
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
		log.Debug().Msgf("form processing error %s", err)
		httperror.RichError(w, r, err)
		return
	}

	// stuff user data in shared map, addressed by uuid
	sharedUserData := sharedUserDataFromContext(r.Context())
	userDataKey, err := sharedUserData.Set(safeUserData)
	if err != nil {
		log.Warn().Msgf("issue storing user data for future internal use %s", err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	defer sharedUserData.Clear(userDataKey)

	q := url.Values{}
	q.Set(InternalHttpServerPdfTemplateRequestUserQueryKey, userDataKey)
	internalTemplateWebHostname := internalTemplateWebHostnameFromContext(r.Context())
	host := fmt.Sprintf("%s:%s", internalTemplateWebHostname, InternalHttpServerPdfTemplatePort)
	pdfUrl := &url.URL{
		Scheme:   "http",
		Host:     host,
		Path:     InternalHttpServerPdfTemplatePath,
		RawQuery: q.Encode(),
	}

	pdfGenerator := pdfGenControlFromContext(r.Context())

	pdfData, err := pdfGenerator.GeneratePdf(ctx, pdfUrl.String())
	if err != nil {
		log.Error().Msgf("error generating PDF: %s", err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Length", strconv.Itoa(len(pdfData)))
	_, err = w.Write(pdfData)
	if err != nil {
		log.Warn().Msgf("writing PDF data failed: %s", err)
	}

	userData := safeUserData.GetUserData()
	log.Info().
		Dur("pdf_gen_duration", time.Since(start)).
		Str("regular_rpps", userData.Regular.NumberRPPS).
		Str("substitute_rpps", userData.Substituting.NumberRPPS).
		Msg("created a contract")
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
			log.Debug().Msgf("invalid doctor search query: %s", err)
			http.Error(w, http.StatusText(http.StatusUnprocessableEntity), http.StatusUnprocessableEntity)
		} else {
			if errors.Is(err, context.Canceled) == false {
				log.Warn().Msgf("unexpected error with doctor search query: %s", err)
			}
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
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	w.Write(b)
}

var newLineRegexp = regexp.MustCompile(`\r?\n`)

const FrontEndErrLogItemLimit = 800

func limitAndReplaceNewlinesWithSpaces(s string) string {
	if len(s) > FrontEndErrLogItemLimit {
		s = s[:FrontEndErrLogItemLimit]
		s += " [...]"
	}
	return newLineRegexp.ReplaceAllString(s, " ")
}

func frontendErrorLogHandler(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	errorEventType := limitAndReplaceNewlinesWithSpaces(r.PostFormValue("eventType"))
	message := limitAndReplaceNewlinesWithSpaces(r.PostFormValue("message"))
	userAgent := limitAndReplaceNewlinesWithSpaces(r.PostFormValue("useragent"))

	if errorEventType == "" || message == "" || userAgent == "" {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	stack := limitAndReplaceNewlinesWithSpaces(r.PostFormValue("stack"))

	var sb strings.Builder
	fmt.Fprintf(&sb, "Front-end issue (of type \"%s\") received from UA \"%s\": \"%s\"", errorEventType, userAgent, message)
	if stack != "" {
		fmt.Fprintf(&sb, ", stack: %s", stack)
	}

	log.Warn().Msg(sb.String())
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
		log.Error().Msgf("error retrieving internal data for PDF: %s", err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	pdfGenerator := pdfGenControlFromContext(r.Context())

	err = pdfGenerator.Template.Execute(w, userData)
	if err != nil {
		log.Error().Msgf("error serving internal PDF template: %s", err)
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
	publicFacingWebsitePathRoot := flag.String("http-data", "", "the directory containing files to host over HTTP")
	drDataFilePath := flag.String("dr-data-file", "", "the file containing the doctor contact data. This should be an extraction from https://annuaire.sante.fr/web/site-pro/extractions-publiques")

	pdfTemplateFilePath := flag.String("pdf-template-file", "", "the HTML file used as a template for contract PDFs")
	pdfGenBrowserDevToolsUrl := flag.String("pdf-browser-devtools-url", PdfGeneratorBrowserDevToolsUrl, "the URL of the browser devtools server to target and control for PDF generation")
	pdfInternalTemplateWebHostname := flag.String("pdf-internal-web-hostname", "", "the hostname that external services should use to access the internal contract template web server")

	// Flags useful when developping.
	devWebsiteProxyPort := flag.String("http-proxy", "", "a port to reverse-proxy the user-facing web HTTP requests (useful for developping front-end)")
	flag.Parse()

	// Global logging setup
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	zerolog.SetGlobalLevel(zerolog.DebugLevel)
	// TODO: use a dev flag instead of this crutch
	if *devWebsiteProxyPort != "" {
		zerolog.SetGlobalLevel(zerolog.TraceLevel)
		log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})
	}

	// Flag checks
	if *drDataFilePath == "" {
		log.Fatal().Msg("a file must be specified for doctor data")
	}
	if *pdfTemplateFilePath == "" {
		log.Fatal().Msg("an HTML file must be specified for PDF template")
	}
	if *pdfGenBrowserDevToolsUrl == "" {
		log.Fatal().Msg("a URL must be specified for the browser devtools")
	}
	if *pdfInternalTemplateWebHostname == "" {
		log.Fatal().Msg("a hostname must be specified for the internal template web host")
	}

	// Load time-zone for Paris.
	parisLocation, err := time.LoadLocation("Europe/Paris")
	if err != nil {
		log.Fatal().Msgf("could not load Paris time zone information %s", err)
	}

	err = SharedPdfGenControl.Init(*pdfTemplateFilePath, *pdfGenBrowserDevToolsUrl, PdfGeneratorInitializationTimeout)
	if err != nil {
		log.Fatal().Msgf("could not initialize PDF sub-system %s", err)
	}
	defer SharedPdfGenControl.Shutdown()

	// Setup doctor search structure.
	SharedDoctorSearcher = doctorsearch.New(*drDataFilePath, DoctorSearchNGramSize, MaxDoctorSearchQueryLength, MaxDoctorSearchConcurrentQueries)

	// Internal HTTP server for use with headless Web browser instance to convert web pages to PDF.
	errChan := make(chan error)
	go func() {
		pdfServeMux := http.NewServeMux()
		pdfServeMux.HandleFunc(InternalHttpServerPdfTemplatePath, withContext(forMethod(http.MethodGet, pdfTemplateHandler)))

		pdfServer := &http.Server{
			// TODO: when not running within a container, we'd ideally want this to be bound to localhost and not to all interfaces.
			Addr:              fmt.Sprintf(":%s", InternalHttpServerPdfTemplatePort),
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
		if *publicFacingWebsitePathRoot != "" {
			rootHandler = http.FileServer(http.Dir(*publicFacingWebsitePathRoot))
		} else if *devWebsiteProxyPort != "" {
			urlToProxyTo, err := url.Parse(fmt.Sprintf("http://localhost:%s/", *devWebsiteProxyPort))
			if err != nil {
				log.Fatal().Msgf("could not use specified proxy URL: %s", err)
			}
			rootHandler = httputil.NewSingleHostReverseProxy(urlToProxyTo)
		} else {
			log.Fatal().Msg("you must specify one of -http-data or -http-proxy flags")
		}
		publicServeMux.Handle("/", csp.WithSecurityHeaders(rootHandler))

		publicServeMux.HandleFunc("/b/generate-contract",
			withContext(
				withInternalTemplateWebHostname(*pdfInternalTemplateWebHostname,
					withTimeZoneLocation(parisLocation,
						forMethod(http.MethodPost,
							genContractHandler)))))

		publicServeMux.HandleFunc("/b/search-doctor",
			withContext(
				forMethod(http.MethodGet,
					doctorSearchHandler)))

		publicServeMux.HandleFunc("/b/log-error",
			withContext(
				forMethod(http.MethodPost,
					frontendErrorLogHandler)))

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

	log.Info().
		Str("port", *publicFacingWebsitePort).
		Msgf("autocontract HTTP service starting on port %s", *publicFacingWebsitePort)
	err = <-errChan
	log.Fatal().Msgf("issue with an HTTP server: %s", err)
}
