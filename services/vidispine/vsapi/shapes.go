package vsapi

import (
	"fmt"
	"net/url"

	"github.com/bcc-code/bcc-media-flows/services/vidispine/vscommon"
	"github.com/samber/lo"
)

func (c *Client) GetShapes(vsID string) (*ShapeResult, error) {
	url := c.baseURL + "/item/" + vsID + "?content=shape&terse=true"

	resp, err := c.restyClient.R().
		SetResult(&ShapeResult{}).
		Get(url)

	if err != nil {
		return nil, err
	}

	return resp.Result().(*ShapeResult), nil
}

// AddShapeToItem creates a job for adding the shape to the item. Returns the job ID.
func (c *Client) AddShapeToItem(tag, itemID, fileID string) (string, error) {
	requestURL, _ := url.Parse(c.baseURL)
	requestURL.Path += "/item/" + url.PathEscape(itemID) + "/shape"
	q := requestURL.Query()
	q.Set("storageId", DefaultStorageID)
	q.Set("fileId", fileID)
	q.Set("tag", tag)
	requestURL.RawQuery = q.Encode()

	result, err := c.restyClient.R().
		SetHeader("Accept", "application/json").
		SetResult(&JobDocument{}).
		Post(requestURL.String())

	if err != nil {
		return "", err
	}

	return result.Result().(*JobDocument).JobID, nil
}

func (c *Client) DeleteShape(assetID, shapeID string) error {
	if shapeID == "" {
		return fmt.Errorf("shapeID is empty - would delete all shapes")
	}

	requestURL, _ := url.Parse(c.baseURL)
	q := requestURL.Query()
	q.Add("keepFiles", "true")
	requestURL.RawQuery = q.Encode()
	requestURL.Path += fmt.Sprintf("/item/%s/shape/%s", url.PathEscape(assetID), url.PathEscape(shapeID))

	_, err := c.restyClient.R().
		Delete(requestURL.String())

	return err
}

// AddSidecarToItem creates a job for adding the sidecar to the item. Returns the job ID.
func (c *Client) AddSidecarToItem(itemID, filePath, language string) (string, error) {
	requestURL, _ := url.Parse(c.baseURL)
	requestURL.Path += "/import/sidecar/" + url.PathEscape(itemID)
	q := requestURL.Query()
	q.Set("sidecar", "file://"+filePath)
	q.Set("jobmetadata", "subtitleLanguage="+language)
	requestURL.RawQuery = q.Encode()

	result, err := c.restyClient.R().
		SetHeader("Accept", "application/json").
		SetResult(&JobDocument{}).
		Post(requestURL.String())

	if err != nil {
		return "", err
	}

	return result.Result().(*JobDocument).JobID, nil
}

func (c *Client) GetResolutions(itemVXID string) ([]Resolution, error) {
	meta, err := c.GetMetadata(itemVXID)
	if err != nil {
		return nil, err
	}

	isSequence := meta.Get(vscommon.FieldSequenceSize, "0") != "0"
	if isSequence {
		seq, err := c.GetSequence(itemVXID)
		if err != nil {
			return nil, err
		}
		for _, t := range seq.Track {
			if t.Audio {
				continue
			}
			if id := t.Segments[0].VXID; id != "" {
				itemVXID = id
				break
			}
		}
	}

	shapes, err := c.GetShapes(itemVXID)
	if err != nil {
		return nil, err
	}

	shape := shapes.GetShape("original")
	if shape == nil {
		return nil, fmt.Errorf("no original shape found")
	}
	if len(shape.VideoComponent) == 0 {
		return nil, nil
	}

	r := shape.VideoComponent[0].Resolution
	var qualities []Resolution
	if r.Width/r.Height == 16/9 {

		if r.Height >= 2160 {
			qualities = append(qualities, Resolution{Width: 3840, Height: 2160})
		}

		if r.Height >= 1080 {
			qualities = append(qualities, Resolution{Width: 1920, Height: 1080})
		}

		if r.Height >= 720 {
			qualities = append(qualities, Resolution{Width: 1280, Height: 720})
		}

		if r.Height >= 560 {
			qualities = append(qualities, Resolution{Width: 960, Height: 540})
			qualities = append(qualities, Resolution{Width: 640, Height: 360})
			qualities = append(qualities, Resolution{Width: 480, Height: 270})
			qualities = append(qualities, Resolution{Width: 320, Height: 180})
		}

	} else {
		qualities = append(qualities, r)
		for {
			r = Resolution{Width: r.Width / 2, Height: r.Height / 2}

			// Make sure stuff is divisible by 2
			if r.Width%2 != 0 {
				r.Width++
			}

			if r.Height%2 != 0 {
				r.Height++
			}

			qualities = append(qualities, r)
			if r.Height < 200 || r.Width < 180 {
				break
			}
		}
	}
	return qualities, nil
}

func (sr ShapeResult) GetShape(tag string) *Shape {
	for _, s := range sr.Shape {
		if lo.Contains(s.Tag, tag) {
			return &s
		}
	}
	return nil
}

func (s Shape) GetPath() string {

	if len(s.ContainerComponent.File) > 0 {
		// Cut off the "file://" prefix
		for _, fc := range s.ContainerComponent.File {
			p, _ := url.PathUnescape(fc.URI[0][7:])
			return p
		}
	}

	// Does this make sense, can it be multiple files???
	for _, bc := range s.BinaryComponent {
		for _, f := range bc.File {
			p, _ := url.PathUnescape(f.URI[0][7:])
			return p
		}
	}

	return ""
}

///// SUPPORTING TYPES /////

type ShapeResult struct {
	Shape []Shape `json:"shape"`
	ID    string  `json:"id"`
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
	Items       []Item   `json:"item,omitempty"`
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

type BinaryComponent struct {
	Length   int    `json:"length"`
	File     []File `json:"file"`
	ID       string `json:"id"`
	Metadata []KV   `json:"metadata"`
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
	BinaryComponent    []BinaryComponent  `json:"binaryComponent"`
	Metadata           KV                 `json:"metadata"`
}

type Timestamp struct {
	Samples  int      `json:"samples"`
	TimeBase Fraction `json:"timeBase"`
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
