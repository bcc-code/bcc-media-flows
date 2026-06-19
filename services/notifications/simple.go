package notifications

import (
	_ "embed"
)

var (
	//go:embed templates/simple_notification.gohtml
	simpleNotificationTemplateFS string
	simpleNotificationTemplate   = mustEmailTemplate("simple_notification", simpleNotificationTemplateFS)
)

type Simple struct {
	Title   string
	Message string
}

func (t Simple) RenderHTML() (string, error) {
	return renderHtmlTemplate(simpleNotificationTemplate, t)
}

func (t Simple) Subject() string {
	return t.Title
}

func (t Simple) RenderMarkdown() (string, error) {
	var markdown string
	if t.Title != "" {
		markdown += "# " + t.Title + "\n\n"
	}
	if t.Message != "" {
		markdown += t.Message
	}
	return markdown, nil
}
