package notifications

import (
	_ "embed"
	"fmt"
)

var (
	//go:embed templates/import_failed.gohtml
	importFailedTemplateString string
	importFailedTemplate       = mustEmailTemplate("import_failed", importFailedTemplateString)
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
	md := "❌ Import failed\n" +
		"Job ID: %s\n" +
		"Error: `%s`\n" +
		"Files:\n%s"

	files := ""
	for _, f := range t.Files {
		files += fmt.Sprintf("- `%s`\n", f.Name)
	}

	return fmt.Sprintf(md, t.JobID, t.Error, files), nil
}

func (t ImportFailed) Subject() string {
	return t.Title
}
