package notifications

import (
	_ "embed"
	"html/template"
)

var (
	//go:embed templates/import_completed.gohtml
	importCompletedTemplateString string
	importCompletedTemplate       = template.Must(template.New("import_completed").Parse(importCompletedTemplateString))
)

type ImportCompleted struct {
	Title string
	JobID string
	Files []File
}

func (t ImportCompleted) RenderHTML() (string, error) {
	return renderHtmlTemplate(importCompletedTemplate, t)
}

func (t ImportCompleted) RenderMarkdown() (string, error) {
	return "", nil
}
