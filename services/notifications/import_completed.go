package notifications

import (
	_ "embed"
	"fmt"
)

var (
	//go:embed templates/import_completed.gohtml
	importCompletedTemplateString string
	importCompletedTemplate       = mustEmailTemplate("import_completed", importCompletedTemplateString)
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
	md := "✅ Import completed\n" +
		"Job ID: %s\n" +
		"Files:\n%s"

	files := ""
	for _, f := range t.Files {
		files += fmt.Sprintf("- `%s`\n", f.Name)
	}

	return fmt.Sprintf(md, t.JobID, files), nil
}

func (t ImportCompleted) Subject() string {
	return t.Title
}
