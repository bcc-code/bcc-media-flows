package vidispine

import (
	"errors"
	"strconv"
	"strings"

	"github.com/davecgh/go-spew/spew"
)

type ExportAudioSource string

type FileWithTS struct {
	FilePath string  `json:"filePath"` // absolute unix path
	StartTS  float64 `json:"startTS"`  // in seconds, usable for ffmpeg
	EndTS    float64 `json:"endTS"`    // in seconds, usable for ffmpeg
}

type ExportData struct {
	Title  string
	Videos []FileWithTS              `json:"videos"`
	Audios map[string]([]FileWithTS) `json:"audios"`
	Subs   map[string]([]FileWithTS) `json:"subs"`
}

const (
	FIELD_EXPORT_AUDIO_SOURCE = "portal_mf452504"
	FIELD_TITLE               = "title"
	FIELD_SUBCLIP_TO_EXPORT   = "portal_mf230973"
	FIELD_START_TC            = "startTimeCode"

	EXPORT_AUDIO_SOURCE_EMBEDDED ExportAudioSource = "embedded"
	EXPORT_AUDIO_SOURCE_RELATED  ExportAudioSource = "related"
)

func TCToSeconds(tc string) (float64, error) {
	splits := strings.Split(tc, "@")
	if len(splits) != 2 {
		return 0, errors.New("Invalid timecode: " + tc)
	}

	samples, err := strconv.ParseFloat(splits[0], 64)
	if err != nil {
		return 0, err
	}

	if splits[1] != "PAL" {
		return 0, errors.New("Invalid timecode. Currently only <NUMBER>@PAL is supported: " + tc)
	}

	// PAL = 25 fps
	// http://10.12.128.15:8080/APIdoc/time.html#time-bases
	return samples / 25, nil
}

// GetInOut returns the in and out point of the clip in seconds, suitable
// for use with ffmpeg
func (m *MetadataResult) GetInOut(beginTC string) (float64, float64, error) {
	var v *MetadataField
	if val, ok := m.Terse["title"]; !ok {
		// This should not happen as everything should have a title
		return 0, 0, errors.New("Missing title")
	} else {
		v = val[0]
	}

	start := 0.0
	if v.Start == "-INF" && v.End == "+INF" {
		// This is a full asset so we return 0.0 start and the lenght of the asset as end
		endString := m.Get("durationSeconds", "0")
		end, err := strconv.ParseFloat(endString, 64) // TODO: Error?
		return start, end, err
	}

	// Now we are in subclip territory. Here we need to extract the TC of the in and out point
	// and convert it to seconds for use with ffmpeg

	inTCseconds, err := TCToSeconds(v.Start)
	if err != nil {
		return 0, 0, err
	}

	outTCseconds, err := TCToSeconds(v.End)
	if err != nil {
		return 0, 0, err
	}

	// This is basically the offset of the tc that we have to remove from the in and out point
	beginTCseconds, err := TCToSeconds(beginTC)
	if err != nil {
		return 0, 0, err
	}

	return inTCseconds - beginTCseconds, outTCseconds - beginTCseconds, nil
}

// GetDataForExport returns the data needed to export the item with the given VXID
// If exportSubclip is true, the subclip will be exported, otherwise the whole clip
func (c *Client) GetDataForExport(itemVXID string) error {
	meta, err := c.GetMetadata(itemVXID)
	if err != nil {
		return err
	}

	metaClips := meta.SplitByClips()

	// Get the metadata for the original clip
	meta = metaClips[ORIGINAL_CLIP]

	// Determine where to take the audio from
	audioSource := EXPORT_AUDIO_SOURCE_EMBEDDED
	if ExportAudioSource(meta.Get(FIELD_EXPORT_AUDIO_SOURCE, "")) == EXPORT_AUDIO_SOURCE_RELATED {
		audioSource = EXPORT_AUDIO_SOURCE_RELATED
	}

	// Check for sequence
	isSequence := meta.Get("__sequence_size", "0") != "0"

	// Check for subclip
	isSubclip := len(metaClips) > 1

	title := meta.Get("title", "")
	exportFormat := meta.Get("portal_mf868653", "NOTHING") // TODO: What is this?

	out := ExportData{
		Title:  title,
		Videos: []FileWithTS{},
		Audios: map[string]([]FileWithTS){},
		Subs:   map[string]([]FileWithTS){},
	}

	if isSequence {
		// TODO: Implement
		return errors.New("Sequences are not supported yet")
	}

	videoOut := FileWithTS{
		StartTS: 0,
		EndTS:   0,
	}

	if isSubclip {
		var subclipMeta *MetadataResult

		subclipName := meta.Get(FIELD_SUBCLIP_TO_EXPORT, "")
		if scMeta, ok := metaClips[subclipName]; ok {
			subclipMeta = scMeta
		} else {
			return errors.New("Subclip " + subclipName + " does not exist")
		}

		in, out, err := subclipMeta.GetInOut(meta.Get(FIELD_START_TC, "0@PAL"))
		if err != nil {
			return err
		}

		videoOut.StartTS = in
		videoOut.EndTS = out
	} else {
		in, out, err := meta.GetInOut("")
		if err != nil {
			return err
		}

		videoOut.StartTS = in
		videoOut.EndTS = out
	}

	/// TODO: Audio files.
	// How do we indicate that the tracks should be taken from the video file?
	// TODO: File path
	// TODO: Subs

	out.Videos = append(out.Videos, videoOut)

	println("Exporting item " + itemVXID)
	spew.Dump(exportFormat)
	spew.Dump(out)
	spew.Dump(audioSource)
	println("---------------" + itemVXID)
	println()

	return nil
}
