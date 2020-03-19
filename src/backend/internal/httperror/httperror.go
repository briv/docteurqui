package httperror

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"autocontract/internal/validation"

	"github.com/golang/gddo/httputil"
)

const (
	NotAcceptable   string = ""
	ApplicationJson string = "application/json"
	TextPlain       string = "text/plain"
	TextHtml        string = "text/html"
)

type genericJSONError struct {
	Message string `json:"error"`
}

func writeError(w http.ResponseWriter, mediaType string, err error) {
	// TODO: log details on server-side
	if mediaType == NotAcceptable {
		http.Error(w, http.StatusText(http.StatusNotAcceptable), http.StatusNotAcceptable)
		return
	}

	w.Header().Set("X-Content-Type-Options", "nosniff")

	switch mediaType {
	case ApplicationJson:
		{
			w.Header().Set("Content-Type", "application/json; charset=utf-8")

			encoder := json.NewEncoder(w)

			var perr validation.UserError
			if errors.As(err, &perr) {
				w.WriteHeader(http.StatusUnprocessableEntity)
				encoder.Encode(perr.Issues)
			} else {
				w.WriteHeader(http.StatusBadRequest)
				encoder.Encode(genericJSONError{
					Message: "Bad HTTP Request",
				})
			}

			break
		}
	// TODO: We take some liberty with the text/html content type and just return
	// plaintext as browsers can handle it.
	// Is it worth having a dedicated response for text/html ?
	case TextPlain, TextHtml:
		{
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			var perr validation.UserError
			if errors.As(err, &perr) {
				w.WriteHeader(http.StatusUnprocessableEntity)
				fmt.Fprintln(w, http.StatusText(http.StatusUnprocessableEntity))
				fmt.Fprintf(w, "\nThe form could not be processed due to the following error:\n")
				fmt.Fprintf(w, "\t%s\n", perr.Error())
			} else {
				w.WriteHeader(http.StatusBadRequest)
				fmt.Fprintln(w, http.StatusText(http.StatusBadRequest))
			}
			break
		}
	}
}

func RichError(w http.ResponseWriter, r *http.Request, err error) {
	availableTypes := []string{ApplicationJson, TextPlain, TextHtml}
	const defaultOffer = NotAcceptable
	mediaType := httputil.NegotiateContentType(r, availableTypes, defaultOffer)
	writeError(w, mediaType, err)
}
