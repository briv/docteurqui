package csp

import (
	"net/http"
	"path"
)

const (
	HeaderCSP                 = "Content-Security-Policy"
	HeaderXContentTypeOptions = "X-Content-Type-Options"

	InternalPdfServerCSPHeader = `default-src 'none'; style-src 'unsafe-inline'; img-src data:`
)

type AddCSPSecurity interface {
	WithSecurityHeaders(http.Handler) http.HandlerFunc
}

type addCSPSecurity struct {
	isProductionMode bool
}

type SecHeadersConfig struct {
	InsecureMode bool
}

func New(config SecHeadersConfig) AddCSPSecurity {
	return &addCSPSecurity{isProductionMode: !config.InsecureMode}
}

func (a *addCSPSecurity) WithSecurityHeaders(h http.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		if a.isProductionMode && path.Clean(req.URL.Path) == "/" {
			w.Header().Set(HeaderCSP, "default-src 'self'")
			w.Header().Set(HeaderXContentTypeOptions, "nosniff")
		}
		h.ServeHTTP(w, req)
	}
}
