// +build dev

package csp

import (
	"github.com/kr/secureheader"
)

var defaultCSPConfig = &secureheader.Config{
	HTTPSRedirect:          false,
	HTTPSUseForwardedProto: false,

	PermitClearLoopback: false,

	ContentTypeOptions: true,

	CSP:          true,
	CSPBody:      "default-src 'self'; connect-src 'self' ws:",
	CSPReportURI: "",

	CSPReportOnly: false,

	HSTS: false,

	FrameOptions:       true,
	FrameOptionsPolicy: secureheader.Deny,

	XSSProtection:      true,
	XSSProtectionBlock: true,
}
