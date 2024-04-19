package notifications

import (
	_ "embed"
	"html/template"
)

var (
	//go:embed templates/simple_notification.gohtml
	simpleNotificationTemplateFS string
	simpleNotificationTemplate   = template.Must(template.New("simple_notification").Parse(simpleNotificationTemplateFS))
)

type Simple struct {
	Title   string
	Message string
}

func (t Simple) RenderHTML() (string, error) {
	return renderHtmlTemplate(simpleNotificationTemplate, t)
}

func (t SimpleNotification) Subject() string {
	return t.Title
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
