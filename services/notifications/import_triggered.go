package notifications

import (
	_ "embed"
	"fmt"
	"strings"
)

var (
	//go:embed templates/import_triggered.gohtml
	importTriggeredTemplateString string
	importTriggeredTemplate       = mustEmailTemplate("import_triggered", importTriggeredTemplateString)
)

// DetailRow is a single label/value pair shown in the import-triggered email.
type DetailRow struct {
	Label string
	Value string
}

// ImportTriggered notifies the uploader that an import has started. It renders
// the order form metadata as a readable table (HTML) and list (plain text)
// instead of dumping the raw order form name.
type ImportTriggered struct {
	OrderForm  string
	Filename   string
	UploadedBy string
	UploadedAt string
	Details    []DetailRow
}

func (t ImportTriggered) RenderHTML() (string, error) {
	return renderHtmlTemplate(importTriggeredTemplate, t)
}

func (t ImportTriggered) RenderMarkdown() (string, error) {
	var b strings.Builder

	fmt.Fprintf(&b, "Import started: %s\n\n", t.OrderForm)
	b.WriteString("Your upload has been received and an import has been started.\n\n")

	if t.Filename != "" {
		fmt.Fprintf(&b, "File: %s\n", t.Filename)
	}
	if t.UploadedBy != "" {
		fmt.Fprintf(&b, "Uploaded by: %s\n", t.UploadedBy)
	}
	if t.UploadedAt != "" {
		fmt.Fprintf(&b, "Uploaded at: %s\n", t.UploadedAt)
	}

	if len(t.Details) > 0 {
		b.WriteString("\nDetails:\n")
		for _, d := range t.Details {
			if d.Value == "" {
				continue
			}
			fmt.Fprintf(&b, "- %s: %s\n", d.Label, d.Value)
		}
	}

	return b.String(), nil
}

func (t ImportTriggered) Subject() string {
	return fmt.Sprintf("Import started: %s", t.OrderForm)
}
