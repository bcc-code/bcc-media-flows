package ingestworkflows

import (
	"testing"

	"github.com/bcc-code/bcc-media-flows/services/ingest"
)

func TestTranslateJSONForm_Masters(t *testing.T) {
	form := ingest.JSONForm{
		OriginalFilename: "wallhaven-3ld95v.jpg",
		Target:           "MD Test 2",
		FormKey:          "masters",
		Fields: map[string]string{
			"episode": "E04",
			"project": "TP01",
			"season":  "S03",
			"title":   "lkjasdlkjdlkj",
		},
		UploaderEmail: "uploader@example.com",
	}

	metadata, orderForm, err := translateJSONForm(form)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if orderForm != OrderFormVBMaster {
		t.Errorf("orderForm = %q, want %q", orderForm.Value, OrderFormVBMaster.Value)
	}

	jp := metadata.JobProperty
	checks := map[string]struct{ got, want string }{
		"OrderForm":        {jp.OrderForm, OrderFormVBMaster.Value},
		"SenderEmail":      {jp.SenderEmail, "uploader@example.com"},
		"ReceivedFilename": {jp.ReceivedFilename, "wallhaven-3ld95v.jpg"},
		"ProgramID":        {jp.ProgramID, "TP01"},
		"Season":           {jp.Season, "S03"},
		"Episode":          {jp.Episode, "E04"},
		"EpisodeTitle":     {jp.EpisodeTitle, "lkjasdlkjdlkj"},
	}
	for name, c := range checks {
		if c.got != c.want {
			t.Errorf("%s = %q, want %q", name, c.got, c.want)
		}
	}
}

func TestTranslateJSONForm_OslofjordDelivery(t *testing.T) {
	form := ingest.JSONForm{
		OriginalFilename: "wallhaven-3lok9v.png",
		Target:           "MD Test",
		FormKey:          "oslofjord_delivery",
		Fields: map[string]string{
			"arrangement": "TC01",
			"navn":        "ASDASDDDASD",
			"post":        "123",
			"subEvent":    "E01",
			"type":        "LED",
		},
		UploaderEmail: "uploader@example.com",
	}

	metadata, orderForm, err := translateJSONForm(form)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if orderForm != OrderFormLEDMaterial {
		t.Errorf("orderForm = %q, want %q", orderForm.Value, OrderFormLEDMaterial.Value)
	}

	jp := metadata.JobProperty
	checks := map[string]struct{ got, want string }{
		"OrderForm":        {jp.OrderForm, OrderFormLEDMaterial.Value},
		"SenderEmail":      {jp.SenderEmail, "uploader@example.com"},
		"ReceivedFilename": {jp.ReceivedFilename, "wallhaven-3lok9v.png"},
		"ProgramID":        {jp.ProgramID, "TC01"},
		"ProgramPost":      {jp.ProgramPost, "123"},
		"AssetType":        {jp.AssetType, "LED"},
		"Episode":          {jp.Episode, "E01"},
		"EpisodeTitle":     {jp.EpisodeTitle, "ASDASDDDASD"},
	}
	for name, c := range checks {
		if c.got != c.want {
			t.Errorf("%s = %q, want %q", name, c.got, c.want)
		}
	}
}

func TestTranslateJSONForm_UnknownFormKey(t *testing.T) {
	_, _, err := translateJSONForm(ingest.JSONForm{FormKey: "nope"})
	if err == nil {
		t.Fatal("expected error for unknown form key, got nil")
	}
}
