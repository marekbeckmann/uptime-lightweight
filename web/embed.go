package web

import "embed"

// DistFS contains the pre-compiled frontend single-page application
//
//go:embed all:dist
var DistFS embed.FS
