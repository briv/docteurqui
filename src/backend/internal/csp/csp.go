package csp

import (
	"net/http"

	"github.com/kr/secureheader"
)

const (
	CSPHeader    = "Content-Security-Policy"
	PdfCSPHeader = `default-src 'none'; style-src 'unsafe-inline'; img-src data:`
)

func WithSecurityHeaders(h http.Handler) http.Handler {
	c := new(secureheader.Config)
	*c = *defaultCSPConfig
	c.Next = h
	return c
}
