package vidispine

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	bccmflows "github.com/bcc-code/bccm-flows"
	mapset "github.com/deckarep/golang-set/v2"
	"github.com/samber/lo"
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
		end, err := strconv.ParseFloat(endString, 64)
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

func getClipForAssetOrSubclip(c *Client, itemVXID string, isSubclip bool, meta *MetadataResult, clipsMeta map[string]*MetadataResult) (*Clip, error) {
	shapes, err := c.GetShapes(itemVXID)
	if err != nil {
		return nil, err
	}

	shape := shapes.GetShape("original")
	if shape == nil {
		return nil, fmt.Errorf("no original shape found for item %s", itemVXID)
	}

	clip := Clip{
		VXID:      itemVXID,
		VideoFile: shape.GetPath(),
	}

	if !isSubclip {
		in, out, err := meta.GetInOut("")
		if err != nil {
			return nil, err
		}
		clip.InSeconds = in
		clip.OutSeconds = out
		return &clip, nil
	}

	var subclipMeta *MetadataResult

	subclipName := meta.Get(FIELD_SUBCLIP_TO_EXPORT, "")
	if scMeta, ok := clipsMeta[subclipName]; ok {
		subclipMeta = scMeta
	} else {
		return nil, errors.New("Subclip " + subclipName + " does not exist")
	}

	in, out, err := subclipMeta.GetInOut(meta.Get(FIELD_START_TC, "0@PAL"))
	clip.InSeconds = in
	clip.OutSeconds = out
	return &clip, err
}

func getRelatedAudios(c *Client, clip *Clip, languagesToExport []string) (*Clip, error) {

	if _, i, ok := lo.FindIndexOf(languagesToExport, func(l string) bool { return l == "nor" }); ok {
		// Move "nor" to the front if available, so we can use it as fallback
		languagesToExport = append(languagesToExport[:i], languagesToExport[i+1:]...)
		languagesToExport = append([]string{"nor"}, languagesToExport...)
	}

	for _, lang := range languagesToExport {

		// Figure out which field holds the related id
		relatedField := bccmflows.LanguagesByISO[lang].RelatedMBFieldID
		if relatedField == "" {
			return clip, errors.New("No related field for language " + lang + ". This indicates missing support in Vidispine")
		}

		// Get metadata for the video clip
		clipMeta, err := c.GetMetadata(clip.VXID)
		if err != nil {
			return clip, err
		}

		// Now we know what audio to export
		relatedAudioVXID := clipMeta.Get(relatedField, "")
		if relatedAudioVXID == "" {
			// If nor (floor language) is missing we fall back to silece
			if lang == "nor" {
				clip.AudioFile[lang] = &AudioFile{
					Channels: []int{1, 2},
					File:     "/mnt/isilon/assets/BlankAudio10h.wav",
				}
			} else if languagesToExport[0] == "nor" {
				// Fall back to "nor" audio and issue a warning *somewhere*
				clip.AudioFile[lang] = clip.AudioFile["nor"]
			}

			continue
		}

		relatedAudioShapes, err := c.GetShapes(relatedAudioVXID)
		if err != nil {
			return clip, err
		}

		// Ok now we can finally get the path to the audio file
		relatedAudioShape := relatedAudioShapes.GetShape("original")
		if relatedAudioShape == nil {
			if languagesToExport[0] == "nor" {
				// Fall back to "nor" audio and issue a warning *somewhere*
				clip.AudioFile[lang] = clip.AudioFile["nor"]
			} else {
				return clip, fmt.Errorf("no original or fallback shape found for item %s", relatedAudioVXID)
			}
		}

		channels := []int{}

		if len(relatedAudioShape.AudioComponent) > 0 {
			for i := 0; i < relatedAudioShape.AudioComponent[0].ChannelCount; i++ {
				channels = append(channels, i+1)
			}
		} else {
			return clip, fmt.Errorf("no audio components found for item %s", relatedAudioVXID)
		}

		clip.AudioFile[lang] = &AudioFile{
			VXID:     relatedAudioVXID,
			File:     relatedAudioShape.GetPath(),
			Channels: channels,
		}

	}

	return clip, nil
}

func getEmbeddedAudio(c *Client, clip *Clip, languagesToExport []string) (*Clip, error) {
	shapes, err := c.GetShapes(clip.VXID)
	if err != nil {
		return clip, err
	}

	shape := shapes.GetShape("original")
	if len(shape.AudioComponent) != 16 && len(shape.AudioComponent) > 2 {
		return clip, fmt.Errorf("Found %d audio components, expected 2 or 16", len(shape.AudioComponent))
	}

	if len(shape.AudioComponent) == 0 {
		// We have no audio, so we fall back to silence
		channels := []int{1, 2}
		for _, lang := range languagesToExport {
			clip.AudioFile[lang] = &AudioFile{
				File:     "/mnt/isilon/assets/BlankAudio10h.wav",
				Channels: channels,
			}
		}

		return clip, nil
	}

	if shape.AudioComponent[0].ChannelCount <= 2 {
		// We have stereo or mono audio, so we copy it to all languages

		channels := []int{}
		for i := 1; i <= shape.AudioComponent[0].ChannelCount; i++ {
			channels = append(channels, i)
		}

		for _, lang := range languagesToExport {
			clip.AudioFile[lang] = &AudioFile{
				VXID:     clip.VXID,
				File:     shape.GetPath(),
				Channels: channels,
			}
		}

	}

	for _, lang := range languagesToExport {
		// We have an actual 16 channel audio file, so we need to figure out which channels to use
		// and assign them to the correct language

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
			return clip, errors.New("No language " + lang + " found in bccmflows.LanguagesByISO")
		}
	}

	return clip, nil
}

// GetDataForExport returns the data needed to export the item with the given VXID
// If exportSubclip is true, the subclip will be exported, otherwise the whole clip
func (c *Client) GetDataForExport(itemVXID string) (*ExportData, error) {
	meta, err := c.GetMetadata(itemVXID)
	if err != nil {
		return nil, err
	}

	// Check for subclip
	// This check needs to happen on the original metadata, not the split one
	isSubclip := len(meta.GetArray("title")) > 1

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

	title := meta.Get("title", "")

	// TODO: This appears to define the shape used for export. Validate how and where this is used
	// exportFormat := meta.Get("portal_mf868653", "original")

	out := ExportData{
		Title: title,
		Clips: []*Clip{},
	}

	// Get the video clips as a base
	if isSequence {
		seq, err := c.GetSequence(itemVXID)
		if err != nil {
			return nil, err
		}
		out.Clips, err = seq.ToClips(c, audioSource)
		if err != nil {
			return nil, err
		}
	} else {
		clip, err := getClipForAssetOrSubclip(c, itemVXID, isSubclip, meta, metaClips)
		if err != nil {
			return nil, err
		}
		out.Clips = append(out.Clips, clip)
	}

	// Process the video clips and get the audio parts
	for _, clip := range out.Clips {
		clip.AudioFile = map[string]*AudioFile{}

		languagesToExport := meta.GetArray(FIELD_LANGS_TO_EXPORT)

		if audioSource == EXPORT_AUDIO_SOURCE_RELATED {
			clip, err = getRelatedAudios(c, clip, languagesToExport)
		} else if audioSource == EXPORT_AUDIO_SOURCE_EMBEDDED {
			clip, err = getEmbeddedAudio(c, clip, languagesToExport)
		}

		if err != nil {
			return nil, err
		}
	}

	allSubLanguages := mapset.NewSet[string]()

	// Fetch subs
	for _, clip := range out.Clips {
		clip.SubFile = map[string]string{}

		// This is independent of audio language export config, we include all subs available
		clipShapes, err := c.GetShapes(clip.VXID)
		if err != nil {
			return nil, err
		}

		for langCode := range bccmflows.LanguagesByISO {
			// There are also videos with .txt subs... we should support those at some point
			shape := clipShapes.GetShape(fmt.Sprintf("sub_%s_srt", langCode))
			if shape == nil || shape.GetPath() == "" {
				continue
			}

			clip.SubFile[langCode] = shape.GetPath()

			// Collect all languages that any of the clips have subs for
			allSubLanguages.Add(langCode)
		}
	}

	for _, clip := range out.Clips {
		// Add empty subs for all languages that any of the clips have subs for if they are missing
		// This is so it is easier to handle down the line if we always have a sub file for all languages
		for langCode := range allSubLanguages.Iter() {
			if _, ok := clip.SubFile[langCode]; !ok {
				clip.SubFile[langCode] = "/mnt/isilon/assets/empty.srt"
			}
		}
	}

	return &out, nil
}