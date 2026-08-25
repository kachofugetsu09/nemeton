package webui

import "embed"

// Assets contains the production Web UI built by Vite.
//
//go:embed all:dist
var Assets embed.FS
