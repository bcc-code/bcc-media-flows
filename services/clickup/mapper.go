package clickup

import "encoding/json"

// Field IDs from the "Shorts Export" list
// (Workspace 9004075864, List 901523232695).
const (
	FieldAssetStatusID     = "a53c5f4d-39ac-4fd0-bbf4-715056e6d495"
	FieldEditorialStatusID = "e634fb82-fcdc-42c8-8179-d004a5df03ed"
	FieldEpisodeID         = "ecdd2ba7-1007-4568-ae27-a5a5c51492d8"
	FieldInID              = "9a21b3ae-9503-4653-a6de-2c82b4f7ed11"
	FieldOutID             = "32402e82-acfe-4af9-a19c-4dcb4cf975c8"
)

// Drop-down option IDs we need to recognise or write.
const (
	OptionAssetStatusDone             = "b77e8099-a8da-4f51-9e14-b328a023d54a"
	OptionEditorialReadyInMediabanken = "13becc99-ccbf-436a-a892-58709acbe6c7"
)

// Drop-down option names — the human-readable values stored on the
// ShortsData struct after mapping.
const (
	AssetStatusDone             = "Done"
	EditorialReadyInMediabanken = "Ready in Mediabanken"
)

// Field returns the custom field with the given ID, or nil if not present.
func (t Task) Field(fieldID string) *CustomField {
	for i := range t.CustomFields {
		if t.CustomFields[i].ID == fieldID {
			return &t.CustomFields[i]
		}
	}
	return nil
}

// ShortText returns the value of a `short_text` custom field, or "" if unset.
func (cf *CustomField) ShortText() string {
	if cf == nil || len(cf.Value) == 0 || string(cf.Value) == "null" {
		return ""
	}
	var s string
	if err := json.Unmarshal(cf.Value, &s); err != nil {
		return ""
	}
	return s
}

// DropDownName returns the human-readable name of the selected option on a
// `drop_down` custom field, or "" if unset. ClickUp stores the value as the
// option's orderindex (0-based integer), so we resolve via type_config.options.
func (cf *CustomField) DropDownName() string {
	if cf == nil || len(cf.Value) == 0 || string(cf.Value) == "null" {
		return ""
	}
	var idx int
	if err := json.Unmarshal(cf.Value, &idx); err != nil {
		return ""
	}
	for _, opt := range cf.TypeConfig.Options {
		if opt.OrderIndex == idx {
			return opt.Name
		}
	}
	return ""
}
