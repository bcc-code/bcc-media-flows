package ingest

// JSONForm is the payload produced by the new upload portal. It replaces the
// XML Metadata order form (see types.go) for newer ingests. Unlike the XML
// form it describes a single uploaded file and carries form-specific values in
// a dynamic Fields map keyed by FormKey.
type JSONForm struct {
	// Filename names the uploaded media file, which sits next to the JSON
	// sidecar in shared storage.
	Filename         string            `json:"filename"`
	OriginalFilename string            `json:"originalFilename"`
	Target           string            `json:"target"`
	FormKey          string            `json:"formKey"`
	Fields           map[string]string `json:"fields"`
	UploaderID       string            `json:"uploaderId"`
	UploaderEmail    string            `json:"uploaderEmail"`
	UploadedAt       string            `json:"uploadedAt"`
}
