package vidispine

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	bccmflows "github.com/bcc-code/bccm-flows"
	"github.com/davecgh/go-spew/spew"
)

type ExportAudioSource string

type Clip struct {
	VideoFile  string
	InSeconds  float64
	OutSeconds float64
	AudioFile  map[string]*AudioFile
	SubFile    map[string]string
	VXID       string
}

type AudioFile struct {
	VXID     string
	Channels []int
	File     string
}

type ExportData struct {
	Clips []*Clip
	Title string
}

const (
	FIELD_EXPORT_AUDIO_SOURCE = "portal_mf452504"
	FIELD_TITLE               = "title"
	FIELD_SUBCLIP_TO_EXPORT   = "portal_mf230973"
	FIELD_LANGS_TO_EXPORT     = "portal_mf326592"
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
		Title: title,
		Clips: []*Clip{},
	}

	if isSequence {
		seq, err := c.GetSequence(itemVXID)
		if err != nil {
			return err
		}
		out.Clips, err = seq.ToClips(c, audioSource)
		if err != nil {
			return err
		}
	} else {

		shapes, err := c.GetShapes(itemVXID)
		if err != nil {
			return err
		}

		shape := shapes.GetShape("original")
		if shape == nil {
			return fmt.Errorf("no original shape found for item %s", itemVXID)
		}

		clip := Clip{
			VXID:      itemVXID,
			VideoFile: shape.GetPath(),
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

			clip.InSeconds = in
			clip.OutSeconds = out
		} else {
			in, out, err := meta.GetInOut("")
			if err != nil {
				return err
			}
			clip.InSeconds = in
			clip.OutSeconds = out
		}

		out.Clips = append(out.Clips, &clip)
	}

	for _, clip := range out.Clips {
		if clip.AudioFile != nil {
			continue
		}

		clip.AudioFile = map[string]*AudioFile{}
		languagesToExport := meta.GetArray(FIELD_LANGS_TO_EXPORT)

		if audioSource == EXPORT_AUDIO_SOURCE_RELATED {
			for _, lang := range languagesToExport {

				// Figure out which field holds the related id
				relatedField := bccmflows.LanguagesByISO[lang].RelatedMBFieldID
				if relatedField == "" {
					return errors.New("No related field for language " + lang + ". This indicates missing support in Vidispine")
				}

				// Get metadata for the video clip
				clipMeta, err := c.GetMetadata(clip.VXID)
				if err != nil {
					return err
				}

				// Now we know what audio to export
				relatedAudioVXID := clipMeta.Get(relatedField, "")
				if relatedAudioVXID == "" {
					// TODO: This should fall back to "nor" audio and issue a warning *somewhere*
					// This is mostly used for example for copyright with music
					return errors.New("No related audio VXID for language " + lang)
				}

				relatedAudioShapes, err := c.GetShapes(relatedAudioVXID)
				if err != nil {
					return err
				}

				// Ok now we can finally get the path to the audio file
				relatedAudioShape := relatedAudioShapes.GetShape("original")
				if relatedAudioShape == nil {
					// TODO: This should fall back to "nor" audio and issue a warning *somewhere*
					return fmt.Errorf("no original shape found for item %s", relatedAudioVXID)
				}

				clip.AudioFile[lang] = &AudioFile{
					VXID:     relatedAudioVXID,
					File:     relatedAudioShape.GetPath(),
					Channels: []int{1, 2}, // TODO: Always stereo? How does ffmpeg number channels/streams?
				}

			}
		} else if audioSource == EXPORT_AUDIO_SOURCE_EMBEDDED {
			// The most common use of this is for exporting a subclip, or a seuence where all clips are from the same source
			// In this case we *assume* that all clips have an embedded audio tracks.

			// TODO: Verify this assumption using ffprobe?

			spew.Dump(clip.VideoFile)
			if !strings.Contains(clip.VideoFile, "MU1") {
				return errors.New("Currently only MU1 is supported with embedded audio. " + clip.VideoFile + " does not contain MU1. Contact support if you need this")
			}

			for _, lang := range languagesToExport {
				if l, ok := bccmflows.LanguagesByISO[lang]; ok {
					channels := []int{}
					for i := 0; i < l.MU1ChannelCount; i++ {
						channels = append(channels, l.MU1ChannelStart+i)
					}

					clip.AudioFile[lang] = &AudioFile{
						VXID:     clip.VXID,
						File:     clip.VideoFile,
						Channels: channels,
					}
				} else {
					// TODO: Warning and fallback to "nor" audio
					return errors.New("No language " + lang + " found in bccmflows.LanguagesByISO")
				}
			}

		}
	}

	/// TODO: Audio files.
	// TODO: Subs

	println("Exporting item " + itemVXID)
	spew.Dump(exportFormat)
	spew.Dump(out)
	spew.Dump(audioSource)
	println("---------------" + itemVXID)
	println()

	return nil
}
