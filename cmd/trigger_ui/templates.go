package main

import (
	"embed"
	"html/template"
)

//go:embed templates/*.gohtml
var templatesFS embed.FS

func parseTemplates() *template.Template {
	return template.Must(template.ParseFS(templatesFS, "templates/*.gohtml"))
}
