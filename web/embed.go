package web

import "embed"

//go:embed all:build fallback.html
var Files embed.FS
