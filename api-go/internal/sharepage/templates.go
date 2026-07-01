package sharepage

import (
	"embed"
	"html/template"
)

//go:embed templates/*.gohtml
var templateFS embed.FS

// pageTemplates holds the parsed share-page ("page") and branded-404
// ("notfound") templates, sharing one "styles" block. Parsing happens exactly
// once at package initialisation — the templates are compiled into the binary
// via go:embed, so this touches no filesystem I/O — never per request.
// html/template (not text/template) is deliberate: its contextual auto-escaping
// is a hard requirement because the application fields come from an external
// provider (PlanIt) and are untrusted.
var pageTemplates = template.Must(template.ParseFS(templateFS, "templates/*.gohtml"))
