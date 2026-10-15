package sharepage

import (
	"embed"
	"html/template"
)

//go:embed templates/*.gohtml
var templateFS embed.FS

// html/template (not text/template) is deliberate: its contextual auto-escaping
// is required because the application fields come from PlanIt and are untrusted.
var pageTemplates = template.Must(template.ParseFS(templateFS, "templates/*.gohtml"))
