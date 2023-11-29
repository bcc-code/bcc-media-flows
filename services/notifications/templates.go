package notifications

import (
	"bytes"
	_ "embed"
	"html/template"
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

//go:embed templates/import_completed.gohtml
var importCompletedTemplateString string

var ImportCompletedTemplate = template.Must(template.New("import_completed").Parse(importCompletedTemplateString))

func (ImportCompleted) IsTemplate() {}

func (t ImportCompleted) RenderHTML() (string, error) {
	return renderHtmlTemplate(ImportCompletedTemplate, t)
}

type SimpleNotification struct {
	Title   string
	Message string
}

//go:embed templates/simple_notification.gohtml
var simpleNotificationTemplateFS string

var SimpleNotificationTemplate = template.Must(template.New("simple_notification").Parse(simpleNotificationTemplateFS))

func (SimpleNotification) IsTemplate() {}
func (t SimpleNotification) RenderHTML() (string, error) {
	return renderHtmlTemplate(SimpleNotificationTemplate, t)
}

func renderHtmlTemplate(t *template.Template, data any) (string, error) {
	var writer bytes.Buffer
	err := t.Execute(&writer, data)
	if err != nil {
		return "", err
	}
	return writer.String(), nil
}
