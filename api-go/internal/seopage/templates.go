package seopage

import (
	"embed"
	"html/template"
)

//go:embed templates/*.gohtml
var templateFS embed.FS

// pageTemplates is parsed once at package initialisation. html/template's
// contextual auto-escaping is required: application text comes from PlanIt and
// is untrusted.
var pageTemplates = template.Must(template.ParseFS(templateFS, "templates/*.gohtml"))
