package assets

import "embed"

//go:embed all:templates
var TemplateFS embed.FS

//go:embed all:static
var StaticFS embed.FS

// Version is set via -ldflags at build time.
// Falls back to "dev" when running with go run.
var Version = "dev"
