package notifications

import (
	_ "embed"
	"html/template"
)

var (
	//go:embed templates/import_failed.gohtml
	importFailedTemplateString string
	importFailedTemplate       = template.Must(template.New("import_failed").Parse(importFailedTemplateString))
)

type ImportFailed struct {
	Title string
	JobID string
	Error string
	Files []File
}

func (t ImportFailed) RenderHTML() (string, error) {
	return renderHtmlTemplate(importFailedTemplate, t)
}

func (t ImportFailed) RenderMarkdown() (string, error) {
	return "", nil
}
