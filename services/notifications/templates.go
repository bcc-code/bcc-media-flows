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

func renderHtmlTemplate(t *template.Template, data any) (string, error) {
	var writer bytes.Buffer
	err := t.Execute(&writer, data)
	if err != nil {
		return "", err
	}
	return writer.String(), nil
}
