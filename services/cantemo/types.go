package cantemo

import "time"

// GetFormats

type GetFormatsResponse struct {
	Formats []Format `json:"formats"`
	Total   int      `json:"total"`
}

type Video struct {
	VideoCodec      string `json:"videoCodec"`
	VideoFPS        string `json:"videoFPS"`
	VideoResolution string `json:"videoResolution"`
	//VideoBitRate    int    `json:"videoBitRate"`
	FieldOrder  string `json:"fieldOrder"`
	PixelFormat string `json:"pixelFormat"`
}
type Audio struct {
	AudioChannelCount int    `json:"audioChannelCount"`
	AudioCodec        string `json:"audioCodec"`
	AudioSamplingRate string `json:"audioSamplingRate"`
	AudioBitRate      int    `json:"audioBitRate"`
}
type DataCorruption struct {
	Corruption bool   `json:"corruption"`
	Message    string `json:"message"`
}
type Storage struct {
	Name string `json:"name"`
	ID   string `json:"id"`
}
type Urls struct {
	ManageShapeMoveURL   string `json:"manage_shape_move_url"`
	ManageShapeCopyURL   string `json:"manage_shape_copy_url"`
	ManageShapeRenameURL string `json:"manage_shape_rename_url"`
	ManageShapeDeleteURL string `json:"manage_shape_delete_url"`
}
type Files struct {
	ID         string  `json:"id"`
	Filename   string  `json:"filename"`
	Filesize   int64   `json:"filesize"`
	FileState  string  `json:"file_state"`
	Manageable bool    `json:"manageable"`
	Storage    Storage `json:"storage"`
	Urls       Urls    `json:"urls"`
}
type Format struct {
	ID             string         `json:"id"`
	Name           string         `json:"name"`
	DisplayName    string         `json:"display_name"`
	Duration       string         `json:"duration"`
	DownloadURI    string         `json:"download_uri"`
	Format         string         `json:"format"`
	MimeType       string         `json:"mimeType"`
	Video          Video          `json:"video"`
	Audio          Audio          `json:"audio"`
	Closed         bool           `json:"closed"`
	DataCorruption DataCorruption `json:"data_corruption"`
	Files          []Files        `json:"files"`
}

// TRANSCRIPTION_JSON

type Transcription struct {
	Text     string     `json:"text"`
	Segments []Segments `json:"segments"`
}
type Words struct {
	Text       string  `json:"text"`
	Start      float64 `json:"start"`
	End        float64 `json:"end"`
	Confidence float64 `json:"confidence"`
}
type Segments struct {
	ID               int     `json:"id"`
	Seek             int     `json:"seek"`
	Start            float64 `json:"start"`
	End              float64 `json:"end"`
	Text             string  `json:"text"`
	Tokens           []int   `json:"tokens"`
	Temperature      float64 `json:"temperature"`
	AvgLogprob       float64 `json:"avg_logprob"`
	CompressionRatio float64 `json:"compression_ratio"`
	NoSpeechProb     float64 `json:"no_speech_prob"`
	Confidence       float64 `json:"confidence"`
	Words            []Words `json:"words"`
}

// META

type ItemMetadata struct {
	ItemType         string          `json:"item_type"`
	MetadataSummary  MetadataSummary `json:"metadata_summary"`
	MediaDetails     MediaDetails    `json:"media_details"`
	ID               string          `json:"id"`
	SystemMetadata   SystemMetadata  `json:"system_metadata"`
	Archived         bool            `json:"archived"`
	Online           bool            `json:"online"`
	Previews         Previews        `json:"previews"`
	Subtitles        []Subtitles     `json:"subtitles"`
	Locked           bool            `json:"locked"`
	AutoPurgeSeconds interface{}     `json:"auto_purge_seconds"`
	OriginalShape    OriginalShape   `json:"original_shape"`
	Permissions      Permissions     `json:"permissions"`
	LatestVersion    int             `json:"latest_version"`
}
type MetadataSummary struct {
	Duration      string      `json:"duration"`
	StartTimecode string      `json:"start_timecode"`
	User          string      `json:"user"`
	Added         string      `json:"added"`
	Type          string      `json:"type"`
	Filename      string      `json:"filename"`
	Format        string      `json:"format"`
	Dimension     string      `json:"dimension"`
	UserFullName  interface{} `json:"user_full_name"`
}
type StartTimecode struct {
	Vidispine string `json:"vidispine"`
	Dropframe bool   `json:"dropframe"`
}
type MediaDetails struct {
	StartTimecode StartTimecode `json:"start_timecode"`
}
type SystemMetadata struct {
	StartTimeCode      string    `json:"startTimeCode"`
	OriginalAudioCodec string    `json:"originalAudioCodec"`
	DurationSeconds    string    `json:"durationSeconds"`
	StartSeconds       string    `json:"startSeconds"`
	OriginalWidth      string    `json:"originalWidth"`
	DurationTimeCode   string    `json:"durationTimeCode"`
	MimeType           string    `json:"mimeType"`
	OriginalHeight     string    `json:"originalHeight"`
	MediaType          string    `json:"mediaType"`
	OriginalFilename   string    `json:"originalFilename"`
	Title              string    `json:"title"`
	Created            time.Time `json:"created"`
	ShapeTag           string    `json:"shapeTag"`
	OriginalVideoCodec string    `json:"originalVideoCodec"`
	OriginalFormat     string    `json:"originalFormat"`
	User               string    `json:"user"`
}
type DurationTimecode struct {
	Vidispine string `json:"vidispine"`
	Dropframe bool   `json:"dropframe"`
}
type Shapes struct {
	URI              string           `json:"uri"`
	Closed           bool             `json:"closed"`
	Growing          bool             `json:"growing"`
	Displayname      string           `json:"displayname"`
	ID               string           `json:"id"`
	DurationTimecode DurationTimecode `json:"duration_timecode"`
}
type AudioTracks struct {
	Name     string `json:"name"`
	URI      string `json:"uri"`
	Language string `json:"language"`
}
type Previews struct {
	Shapes      []Shapes      `json:"shapes"`
	AudioTracks []AudioTracks `json:"audio_tracks"`
}
type Subtitles struct {
	Code string `json:"code"`
	Name string `json:"name"`
}
type OriginalShape struct {
	ID          string `json:"id"`
	DisplayName string `json:"display_name"`
	Closed      bool   `json:"closed"`
}
type Permissions struct {
	GenericReadPermission   bool `json:"generic_read_permission"`
	GenericWritePermission  bool `json:"generic_write_permission"`
	URIReadPermission       bool `json:"uri_read_permission"`
	URIWritePermission      bool `json:"uri_write_permission"`
	MetadataReadPermission  bool `json:"metadata_read_permission"`
	MetadataWritePermission bool `json:"metadata_write_permission"`
}

// GetFiles

type GetFilesResult struct {
	Objects []Objects `json:"objects"`
	Meta    Meta      `json:"meta"`
}

type Item struct {
	ID  string `json:"id"`
	URL string `json:"url"`
}

type Metadata struct {
}

type Objects struct {
	ID                 string   `json:"id"`
	Path               string   `json:"path"`
	Parent             string   `json:"parent"`
	Name               string   `json:"name"`
	ExcludeFromListing bool     `json:"exclude_from_listing"`
	ParentID           string   `json:"parent_id"`
	URI                string   `json:"uri"`
	State              string   `json:"state"`
	Size               int      `json:"size"`
	Hash               string   `json:"hash"`
	TimestampRaw       string   `json:"timestamp"`
	RefreshFlag        bool     `json:"refreshFlag"`
	StorageName        string   `json:"storage_name"`
	Storage            string   `json:"storage"`
	Item               Item     `json:"item"`
	Metadata           Metadata `json:"metadata"`
	ItemType           string   `json:"item_type"`
	VidispineID        string   `json:"vidispine_id"`
	Type               string   `json:"type"`
	Timestamp          time.Time
}

type Meta struct {
	HasNext        bool `json:"has_next"`
	HasPrevious    bool `json:"has_previous"`
	HasOtherPages  bool `json:"has_other_pages"`
	Next           int  `json:"next"`
	Previous       int  `json:"previous"`
	Hits           int  `json:"hits"`
	FirstOnPage    int  `json:"first_on_page"`
	LastOnPage     int  `json:"last_on_page"`
	ResultsPerPage int  `json:"results_per_page"`
	Page           int  `json:"page"`
	Pages          int  `json:"pages"`
}
