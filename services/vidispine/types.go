package vidispine

type MetadataField struct {
	End   string `json:"end"`
	Start string `json:"start"`
	UUID  string `json:"uuid"`
	Value string `json:"value"`
}

type MetadataResult struct {
	Terse map[string]([]*MetadataField) `json:"terse"`
	ID    string                        `json:"id"`
}

// Get returns the first value of the given key, or the fallback if the key is not present
// It does not check what clip the metadata belongs to!
func (m *MetadataResult) Get(key string, fallback string) string {
	if val, ok := m.Terse[key]; !ok {
		return fallback
	} else if len(val) == 0 {
		return fallback
	} else {
		return val[0].Value
	}
}

func (m *MetadataResult) GetArray(key string) []string {
	if val, ok := m.Terse[key]; !ok {
		return []string{}
	} else {
		out := []string{}
		for _, v := range val {
			out = append(out, v.Value)
		}
		return out
	}
}

// Shape Result
type ShapeResult struct {
	Shape []Shape `json:"shape"`
	ID    string  `json:"id"`
}

type Timestamp struct {
	Samples  int      `json:"samples"`
	TimeBase Fraction `json:"timeBase"`
}

type KV struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type File struct {
	ID          string   `json:"id"`
	Path        string   `json:"path"`
	URI         []string `json:"uri"`
	State       string   `json:"state"`
	Size        int64    `json:"size"`
	Hash        string   `json:"hash"`
	Timestamp   string   `json:"timestamp"`
	RefreshFlag int      `json:"refreshFlag"`
	Storage     string   `json:"storage"`
	Metadata    KV       `json:"metadata"`
}

type ContainerComponent struct {
	Duration           Timestamp `json:"duration"`
	Format             string    `json:"format"`
	FirstSMPTETimecode string    `json:"firstSMPTETimecode"`
	StartTimecode      int       `json:"startTimecode"`
	StartTimestamp     Timestamp `json:"startTimestamp"`
	RoundedTimeBase    int       `json:"roundedTimeBase"`
	DropFrame          bool      `json:"dropFrame"`
	TimeCodeTimeBase   Fraction  `json:"timeCodeTimeBase"`
	MediaInfo          MediaInfo `json:"mediaInfo"`
	File               []File    `json:"file"`
	ID                 string    `json:"id"`
	Metadata           []KV      `json:"metadata"`
}

type AudioComponent struct {
	ChannelCount    int       `json:"channelCount"`
	ChannelLayout   int       `json:"channelLayout"`
	SampleFormat    string    `json:"sampleFormat"`
	FrameSize       int       `json:"frameSize"`
	MediaInfo       MediaInfo `json:"mediaInfo"`
	File            []File    `json:"file"`
	ID              string    `json:"id"`
	Metadata        []KV      `json:"metadata"`
	Codec           string    `json:"codec"`
	TimeBase        Fraction  `json:"timeBase"`
	ItemTrack       string    `json:"itemTrack"`
	EssenceStreamID int       `json:"essenceStreamId"`
	Bitrate         int       `json:"bitrate"`
	Pid             int       `json:"pid"`
	Duration        Timestamp `json:"duration"`
	StartTimestamp  Timestamp `json:"startTimestamp"`
}

type Resolution struct {
	Width  int `json:"width"`
	Height int `json:"height"`
}

type Fraction struct {
	Numerator   int `json:"numerator"`
	Denominator int `json:"denominator"`
}

type ContainerSAR struct {
	Horizontal int `json:"horizontal"`
	Vertical   int `json:"vertical"`
}

type MediaInfo struct {
	FormatSettingsGOP string `json:"Format_Settings_GOP"`
	IntraDcPrecision  int    `json:"intra_dc_precision"`
	Property          []KV   `json:"property"`
	BitRateMode       string `json:"Bit_rate_mode"`
}

type VideoComponent struct {
	Resolution           Resolution   `json:"resolution"`
	PixelFormat          string       `json:"pixelFormat"`
	MaxBFrames           int          `json:"maxBFrames"`
	PixelAspectRatio     Fraction     `json:"pixelAspectRatio"`
	FieldOrder           string       `json:"fieldOrder"`
	CodecTimeBase        Fraction     `json:"codecTimeBase"`
	AverageFrameRate     Fraction     `json:"averageFrameRate"`
	RealBaseFrameRate    Fraction     `json:"realBaseFrameRate"`
	DisplayWidth         Fraction     `json:"displayWidth"`
	DisplayHeight        Fraction     `json:"displayHeight"`
	ContainerSAR         ContainerSAR `json:"containerSAR"`
	ColrPrimaries        int          `json:"colr_primaries"`
	ColrTransferFunction int          `json:"colr_transfer_function"`
	ColrMatrix           int          `json:"colr_matrix"`
	StartTimecode        int          `json:"startTimecode"`
	DropFrame            bool         `json:"dropFrame"`
	BitDepth             int          `json:"bitDepth"`
	BitsPerPixel         int          `json:"bitsPerPixel"`
	ColorPrimaries       string       `json:"colorPrimaries"`
	MediaInfo            MediaInfo    `json:"mediaInfo"`
	File                 []File       `json:"file"`
	ID                   string       `json:"id"`
	Metadata             []KV         `json:"metadata"`
	Codec                string       `json:"codec"`
	TimeBase             Fraction     `json:"timeBase"`
	ItemTrack            string       `json:"itemTrack"`
	EssenceStreamID      int          `json:"essenceStreamId"`
	Bitrate              int          `json:"bitrate"`
	NumberOfPackets      int          `json:"numberOfPackets"`
	Extradata            string       `json:"extradata"`
	Pid                  int          `json:"pid"`
	Duration             Timestamp    `json:"duration"`
	Profile              int          `json:"profile"`
	Level                int          `json:"level"`
	StartTimestamp       Timestamp    `json:"startTimestamp"`
}

type Shape struct {
	ID                 string             `json:"id"`
	Created            string             `json:"created"`
	EssenceVersion     int                `json:"essenceVersion"`
	Tag                []string           `json:"tag"`
	MimeType           []string           `json:"mimeType"`
	ContainerComponent ContainerComponent `json:"containerComponent"`
	AudioComponent     []AudioComponent   `json:"audioComponent"`
	VideoComponent     []VideoComponent   `json:"videoComponent"`
	Metadata           KV                 `json:"metadata"`
}
