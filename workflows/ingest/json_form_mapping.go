package ingestworkflows

import (
	"fmt"

	"github.com/bcc-code/bcc-media-flows/services/ingest"
)

// jsonFormSpec describes how a single JSON form (identified by formKey) maps
// onto the existing ingest.Metadata / OrderForm model used by the XML pipeline.
type jsonFormSpec struct {
	orderForm OrderForm
	// fields maps a JSON `fields` key to a pointer to the JobProperty field it
	// populates. Keys absent from the payload are simply left empty.
	fields func(jp *ingest.JobProperty) map[string]*string
}

// jsonFormSpecs is the static registry of supported JSON form keys. Add new
// form keys here; an unknown key is rejected by translateJSONForm.
var jsonFormSpecs = map[string]jsonFormSpec{
	"masters": {
		orderForm: OrderFormVBMaster,
		fields: func(jp *ingest.JobProperty) map[string]*string {
			return map[string]*string{
				"project": &jp.ProgramID,
				"season":  &jp.Season,
				"episode": &jp.Episode,
				"title":   &jp.EpisodeTitle,
			}
		},
	},
	"oslofjord_delivery": {
		orderForm: OrderFormLEDMaterial,
		fields: func(jp *ingest.JobProperty) map[string]*string {
			return map[string]*string{
				"arrangement": &jp.ProgramID,
				"post":        &jp.ProgramPost,
				"type":        &jp.AssetType,
				"subEvent":    &jp.Episode,
				"navn":        &jp.EpisodeTitle,
			}
		},
	},
}

// translateJSONForm converts a JSONForm from the new upload portal into the
// ingest.Metadata + OrderForm consumed by the existing child workflows. Only
// JobProperty is populated (no FileList) since JSON forms describe a single
// file that is passed separately as SourceFile.
func translateJSONForm(form ingest.JSONForm) (*ingest.Metadata, OrderForm, error) {
	spec, ok := jsonFormSpecs[form.FormKey]
	if !ok {
		return nil, OrderForm{}, fmt.Errorf("unsupported form key: %s", form.FormKey)
	}

	jp := ingest.JobProperty{
		OrderForm:        spec.orderForm.Value,
		SenderEmail:      form.UploaderEmail,
		ReceivedFilename: form.OriginalFilename,
	}

	for key, target := range spec.fields(&jp) {
		if value, ok := form.Fields[key]; ok {
			*target = value
		}
	}

	return &ingest.Metadata{JobProperty: jp}, spec.orderForm, nil
}
