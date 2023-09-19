package ingest

type Data struct {
	Duration     string `json:"duration"`
	Files        []File `json:"files"`
	Id           string `json:"id"`
	SmilFile     string `json:"smil_File"`
	Title        string `json:"title"`
	ChaptersFile string `json:"chapters_file"`
}

type File struct {
	Mime             string `json:"mime"`
	Path             string `json:"path"`
	AudioLanguage    string `json:"audiolanguage"`
	SubtitleLanguage string `json:"subtitlelanguage"`
	Resolution       string `json:"resolution"`
}
