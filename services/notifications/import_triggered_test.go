package notifications

import (
	"strings"
	"testing"
)

func sampleOslofjord() ImportTriggered {
	return ImportTriggered{
		OrderForm:  "LED-Material",
		Filename:   "wallhaven-3lok9v.png",
		UploadedBy: "uploader@example.com",
		UploadedAt: "2026-06-19T10:30:00Z",
		Details: []DetailRow{
			{Label: "Name", Value: "Sommerstevne 2026"},
			{Label: "Event", Value: "TC01"},
			{Label: "Sub-event", Value: "E01"},
			{Label: "Post", Value: "123"},
			{Label: "Type", Value: "LED"},
		},
	}
}

func TestImportTriggered_Subject(t *testing.T) {
	if got, want := sampleOslofjord().Subject(), "Import started"; got != want {
		t.Errorf("Subject() = %q, want %q", got, want)
	}
}

func TestImportTriggered_RenderMarkdown(t *testing.T) {
	got, err := sampleOslofjord().RenderMarkdown()
	if err != nil {
		t.Fatalf("RenderMarkdown() error: %v", err)
	}

	for _, want := range []string{
		"Import started",
		"File: wallhaven-3lok9v.png",
		"Uploaded by: uploader@example.com",
		"- Name: Sommerstevne 2026",
		"- Event: TC01",
		"- Type: LED",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("plain text missing %q\n---\n%s", want, got)
		}
	}
}

func TestImportTriggered_RenderMarkdownSkipsEmptyDetails(t *testing.T) {
	n := ImportTriggered{
		OrderForm: "LED-Material",
		Details: []DetailRow{
			{Label: "Name", Value: "Present"},
			{Label: "Post", Value: ""},
		},
	}
	got, err := n.RenderMarkdown()
	if err != nil {
		t.Fatalf("RenderMarkdown() error: %v", err)
	}
	if strings.Contains(got, "Post") {
		t.Errorf("empty detail row should be omitted, got:\n%s", got)
	}
}

func TestImportTriggered_RenderHTML(t *testing.T) {
	got, err := sampleOslofjord().RenderHTML()
	if err != nil {
		t.Fatalf("RenderHTML() error: %v", err)
	}

	for _, want := range []string{
		"<!DOCTYPE html>",
		"Import started",
		"LED-Material",
		"wallhaven-3lok9v.png",
		"Sommerstevne 2026",
		"Sub-event",
		"uploader@example.com",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("HTML missing %q", want)
		}
	}
}
