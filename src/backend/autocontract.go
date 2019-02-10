package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strconv"
	"time"

	"autocontract/internal/datamap"
	"autocontract/internal/pdfgen"
)

const (
	PublicFacingWebsitePort = "18080"

	TimeLayout              = "2006-01-02"
	ParseFormMaxMemoryBytes = 500 * 1024
)

const (
	PdfGeneratorInitializationTimeout = 5 * time.Second
	PdfGeneratorBrowserDevToolsUrl    = "http://localhost:9222"
	PdfGenerationTimeout              = 10 * time.Second
	PurePdfGenerationTimeoutDiff      = 1 * time.Second

	InternalHttpServerPdfTemplatePath                = "/pdf"
	InternalHttpServerPdfTemplatePort                = "18081"
	InternalHttpServerPdfTemplateRequestUserQueryKey = "userDataId"

	ContextUserDataMapKey      = 0
	ContextPdfGenControlKey    = 1
	ContextTimeZoneLocationKey = 2
)

var (
	SharedUserData      = datamap.NewDataMap()
	SharedPdfGenControl = &pdfgen.Control{}
)

func sharedUserDataFromContext(ctx context.Context) datamap.DataMap {
	return ctx.Value(ContextUserDataMapKey).(datamap.DataMap)
}

func pdfGenControlFromContext(ctx context.Context) *pdfgen.Control {
	return ctx.Value(ContextPdfGenControlKey).(*pdfgen.Control)
}

func timeZoneLocationFromContext(ctx context.Context) *time.Location {
	return ctx.Value(ContextTimeZoneLocationKey).(*time.Location)
}

func withContext(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		var ctx context.Context
		ctx = context.WithValue(req.Context(), ContextUserDataMapKey, SharedUserData)
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

	err := r.ParseMultipartForm(ParseFormMaxMemoryBytes)
	if err != nil {
		// TODO: log this error ?
		log.Println(err)
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	var periods []datamap.Period
	timeLocation := timeZoneLocationFromContext(ctx)
	periodStartsStr := r.PostForm["period-start"]
	periodEndsStr := r.PostForm["period-end"]
	for index, periodStartStr := range periodStartsStr {
		periodStart, err := time.Parse(TimeLayout, periodStartStr)
		if err != nil {
			log.Printf("invalid period start '%s'\n", periodStartStr)
			http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
			return
		}

		if index >= len(periodEndsStr) {
			log.Printf("invalid periods (%d starts, %d ends)\n", len(periodStartsStr), len(periodEndsStr))
			http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
			return
		}

		periodEndStr := periodEndsStr[index]
		periodEnd, err := time.Parse(TimeLayout, periodEndStr)
		if err != nil {
			log.Printf("invalid period end '%s'\n", periodEndStr)
			http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
			return
		}

		periods = append(periods, datamap.Period{
			Start: periodStart.In(timeLocation),
			End:   periodEnd.In(timeLocation),
		})
	}

	replacedDoctor := datamap.Person{
		Name:             r.PostFormValue("regular-name"),
		HonorificTitle:   datamap.Docteur,
		NumberRPPS:       r.PostFormValue("regular-rpps"),
		NumberADELI:      r.PostFormValue("regular-adeli"),
		Address:          r.PostFormValue("regular-address"),
		SignatureImgHtml: r.PostFormValue("regular-signature"),
	}

	substituting := datamap.Person{
		Name:                 r.PostFormValue("substitute-name"),
		HonorificTitle:       r.PostFormValue("substitute-title"),
		NumberRPPS:           r.PostFormValue("substitute-rpps"),
		NumberSubstitutingID: r.PostFormValue("substitute-substitutingID"),
		Address:              r.PostFormValue("substitute-address"),
		SignatureImgHtml:     r.PostFormValue("substitute-signature"),
	}

	log.Println(r.PostForm)

	retrocessionPercStr := r.PostFormValue("financials-retrocession")
	retrocessionPerc, err := strconv.Atoi(retrocessionPercStr)
	if err != nil {
		log.Printf("invalid retrocession value '%s'\n", retrocessionPercStr)
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	nightShiftRetrocessionPerc := retrocessionPerc
	nightShiftRetrocessionPercStr := r.PostFormValue("financials-nightShiftRetrocession")
	if nightShiftRetrocessionPercStr != "" {
		nightShiftRetrocessionPerc, err = strconv.Atoi(nightShiftRetrocessionPercStr)
		if err != nil {
			log.Println(err)
			http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
			return
		}
	}

	retrocessionsDiffer := retrocessionPerc != nightShiftRetrocessionPerc
	financials := datamap.Financials{
		HonorairesPercentage: retrocessionPerc,
		Gardes: datamap.GardesFinancials{
			Differs:              retrocessionsDiffer,
			HonorairesPercentage: nightShiftRetrocessionPerc,
		},
	}

	userData := &datamap.UserData{
		Replaced:                replacedDoctor,
		Substituting:            substituting,
		Periods:                 periods,
		Financials:              financials,
		DateContractEstablished: time.Now().In(timeLocation),
	}

	safeUserData, err := datamap.SanitizeUserData(userData)
	if err != nil {
		log.Println(err)
		http.Error(w, http.StatusText(http.StatusUnprocessableEntity), http.StatusUnprocessableEntity)
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
	_, err = w.Write(pdfData)
	if err != nil {
		log.Println(err)
	}

	elapsed := time.Since(start)
	log.Printf("Generating PDF from user data took %s", elapsed)
}

func pdfTemplateHandler(w http.ResponseWriter, r *http.Request) {
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
		if *devWebsiteProxyPort == "" {
			publicServeMux.Handle("/", http.FileServer(http.Dir(*publicFacingWebsitePathRoot)))
		} else {
			urlToProxyTo, err := url.Parse(fmt.Sprintf("http://localhost:%s/", *devWebsiteProxyPort))
			if err != nil {
				log.Fatal(err)
			}
			publicServeMux.Handle("/", httputil.NewSingleHostReverseProxy(urlToProxyTo))
		}

		publicServeMux.HandleFunc("/generate-contract", withContext(
			withTimeZoneLocation(parisLocation, forMethod(http.MethodPost, genContractHandler))))

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
