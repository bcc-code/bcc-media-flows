package ingest

// Metadata is the order form an ingest is described by, and the shape the
// child workflows consume. It was originally the FileCatalyst XML sidecar;
// JSONForm is now translated into it (see translateJSONForm).
type Metadata struct {
	JobProperty JobProperty
}

// JobProperty carries the form fields. Only the fields a form actually fills
// in are set — the JSON forms populate a handful each, and the rest stay
// empty.
type JobProperty struct {
	JobID              int
	OrderForm          string
	AssetType          string
	SenderEmail        string
	EpisodeTitle       string
	EpisodeDescription string
	ProgramPost        string
	ProgramID          string
	Season             string
	Episode            string
	ReceivedFilename   string
	PersonsAppearing   string
	Tags               string
	Language           string
}
