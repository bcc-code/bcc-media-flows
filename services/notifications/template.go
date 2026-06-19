package notifications

import (
	_ "embed"
	"html/template"
)

type Template interface {
	RenderHTML() (string, error)
	RenderMarkdown() (string, error)
	Subject() string
}

//go:embed templates/partials.gohtml
var emailPartials string

// mustEmailTemplate parses a notification body together with the shared
// "header"/"footer" partials so every email shares one branded, email-client
// safe layout. The body is the loose content of template `name`; it invokes
// the partials via {{template "header" .}} / {{template "footer" .}}.
func mustEmailTemplate(name, body string) *template.Template {
	return template.Must(template.New(name).Parse(emailPartials + body))
}
