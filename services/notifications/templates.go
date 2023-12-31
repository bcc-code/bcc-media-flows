package notifications

import (
	"bytes"
	_ "embed"
	"html/template"
)

var (
	//go:embed templates/import_completed.gohtml
	importCompletedTemplateString string
	importCompletedTemplate       = template.Must(template.New("import_completed").Parse(importCompletedTemplateString))

	//go:embed templates/simple_notification.gohtml
	simpleNotificationTemplateFS string
	simpleNotificationTemplate   = template.Must(template.New("simple_notification").Parse(simpleNotificationTemplateFS))
)

type File struct {
	VXID string
	Name string
}

type ImportCompleted struct {
	Title string
	JobID string
	Files []File
}

func (ImportCompleted) IsTemplate() {}

func (t ImportCompleted) RenderHTML() (string, error) {
	return renderHtmlTemplate(importCompletedTemplate, t)
}

func (t ImportCompleted) RenderMarkdown() (string, error) {
	return "", nil
}

type SimpleNotification struct {
	Title   string
	Message string
}

func (SimpleNotification) IsTemplate() {}
func (t SimpleNotification) RenderHTML() (string, error) {
	return renderHtmlTemplate(simpleNotificationTemplate, t)
}

func (t SimpleNotification) RenderMarkdown() (string, error) {
	var markdown string
	if t.Title != "" {
		markdown += "#" + t.Title + "\n\n"
	}
	if t.Message != "" {
		markdown += t.Message
	}
	return markdown, nil
}

func renderHtmlTemplate(t *template.Template, data any) (string, error) {
	var writer bytes.Buffer
	err := t.Execute(&writer, data)
	if err != nil {
		return "", err
	}
	return writer.String(), nil
}
