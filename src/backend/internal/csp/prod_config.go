// +build !devmode

package csp

import (
	"time"

	"github.com/kr/secureheader"
)

var defaultCSPConfig = &secureheader.Config{
	HTTPSRedirect:          true,
	HTTPSUseForwardedProto: false,

	PermitClearLoopback: false,

	ContentTypeOptions: true,

	CSP:          true,
	CSPBody:      "default-src 'self'",
	CSPReportURI: "",

	CSPReportOnly: false,

	HSTS:                  true,
	HSTSMaxAge:            5 * 24 * time.Hour,
	HSTSIncludeSubdomains: true,
	HSTSPreload:           false,

	FrameOptions:       true,
	FrameOptionsPolicy: secureheader.Deny,

	XSSProtection:      true,
	XSSProtectionBlock: true,
}
