package vidispine

import (
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	bccmflows "github.com/bcc-code/bcc-media-flows"
	"github.com/bcc-code/bcc-media-flows/environment"
	"github.com/bcc-code/bcc-media-flows/services/vidispine/vsapi"
	"github.com/bcc-code/bcc-media-flows/services/vidispine/vscommon"
	mapset "github.com/deckarep/golang-set/v2"
	"github.com/orsinium-labs/enum"
	"github.com/samber/lo"
)

var nonAlphanumeric = regexp.MustCompile("[^a-zA-Z0-9_]+")
var consecutiveUnderscores = regexp.MustCompile("_+")

type Clip struct {
	VideoFile          string
	InSeconds          float64
	OutSeconds         float64
	SequenceIn         float64
	SequenceOut        float64
	AudioFiles         map[string]*AudioFile
	SubtitleFiles      map[string]string
	JSONTranscriptFile string
	VXID               string
}

type AudioFile struct {
	VXID    string
	Streams []int
	File    string
}

type ExportData struct {
	Clips []*Clip

	// SafeTitle is a title that can be used in a filename
	SafeTitle string

	// Title is the original title containing spaces and other characters
	Title string

	// ImportDate is the date the asset was imported into Vidispine
	ImportDate *time.Time

	// BmmTitle is the title to use in BMM
	BmmTitle *string

	// BmmTrackID is the track ID in BMM
	BmmTrackID *int

	// If there is a recorded language set in the main clip we take that, otherwise we fall back to Norwegian
	OriginalLanguage string

	TranscribedLanguage string
}

type ExportAudioSource enum.Member[string]

var (
	ExportAudioSourceEmbedded = ExportAudioSource{"embedded"}
	ExportAudioSourceRelated  = ExportAudioSource{"related"}
	ExportAudioSources        = enum.New(
		ExportAudioSourceEmbedded,
		ExportAudioSourceRelated,
	)

	EmptyWAVFile = environment.GetIsilonPrefix() + "/system/assets/BlankAudio10h.wav"
	EmtpySRTFile = environment.GetIsilonPrefix() + "/system/assets/empty.srt"
)

// GetRelatedAudioPaths returns all related audio paths for a given VXID
// Must be separate files.
func GetRelatedAudioPaths(client Client, vxID string) (map[string]string, error) {
	clipMeta, err := client.GetMetadata(vxID)
	if err != nil {
		return nil, err
	}

	var result = map[string]string{}
	for _, lang := range bccmflows.LanguagesByISO {
		relatedField := lang.RelatedMBFieldID
		if relatedField == "" {
			continue
		}

		relatedAudioVXID := clipMeta.Get(vscommon.FieldType{Value: relatedField}, "")
		if relatedAudioVXID == "" {
			continue
		}

		shapes, err := client.GetShapes(relatedAudioVXID)
		if err != nil {
			return nil, err
		}

		shape := shapes.GetShape("original")
		if shape == nil {
			continue
		}
		result[lang.ISO6391] = shape.GetPath()
	}
	return result, nil
}

// enrichClipWithRelatedAudios modifies the clip in-place
//
// TODO: return audiofiles instead of modifying original
func enrichClipWithRelatedAudios(client Client, clip *Clip, oLanguagesToExport []string) error {
	languagesToExport := make([]string, len(oLanguagesToExport))
	copy(languagesToExport, oLanguagesToExport)

	if _, i, ok := lo.FindIndexOf(languagesToExport, func(l string) bool { return l == "nor" }); ok {
		// Move "nor" to the front if available, so we can use it as fallback
		languagesToExport = append(languagesToExport[:i], languagesToExport[i+1:]...)
		languagesToExport = append([]string{"nor"}, languagesToExport...)
	}

	for _, lang := range languagesToExport {

		if lang == "und" {
			continue
		}

		// Figure out which field holds the related id
		relatedField := bccmflows.LanguagesByISO[lang].RelatedMBFieldID
		if relatedField == "" {
			return errors.New("No related field for language " + lang + ". This indicates missing support in Vidispine")
		}

		// Get metadata for the video clip
		clipMeta, err := client.GetMetadata(clip.VXID)
		if err != nil {
			return err
		}

		// Now we know what audio to export
		relatedAudioVXID := clipMeta.Get(vscommon.FieldType{Value: relatedField}, "")
		if relatedAudioVXID == "" {
			// If nor (floor language) is missing we fall back to silece
			if lang == "nor" {
				clip.AudioFiles[lang] = &AudioFile{
					Streams: []int{0},
					File:    EmptyWAVFile,
				}
			} else if languagesToExport[0] == "nor" {
				// Fall back to "nor" audio and issue a warning *somewhere*
				clip.AudioFiles[lang] = clip.AudioFiles["nor"]
			}

			continue
		}

		relatedAudioShapes, err := client.GetShapes(relatedAudioVXID)
		if err != nil {
			return err
		}

		// Ok now we can finally get the path to the audio file
		relatedAudioShape := relatedAudioShapes.GetShape("original")
		if relatedAudioShape == nil {
			if languagesToExport[0] == "nor" {
				// Fall back to "nor" audio and issue a warning *somewhere*
				clip.AudioFiles[lang] = clip.AudioFiles["nor"]
			} else {
				return fmt.Errorf("no original or fallback shape found for item %s", relatedAudioVXID)
			}
			continue
		}

		var streams []int

		if len(relatedAudioShape.AudioComponent) > 0 {
			streams = append(streams, relatedAudioShape.AudioComponent[0].EssenceStreamID)
		} else {
			return fmt.Errorf("no audio components found for item %s", relatedAudioVXID)
		}

		clip.AudioFiles[lang] = &AudioFile{
			VXID:    relatedAudioVXID,
			File:    relatedAudioShape.GetPath(),
			Streams: streams,
		}
	}

	return nil
}

// enrichClipWithEmbeddedAudio modifies the clip in-place with embedded audio
//
// TODO: return audiofiles instead of modifying original
func enrichClipWithEmbeddedAudio(client Client, clip *Clip, languagesToExport []string) error {
	shapes, err := client.GetShapes(clip.VXID)
	if err != nil {
		return err
	}

	shape := shapes.GetShape("original")
	if len(shape.AudioComponent) != 16 && len(shape.AudioComponent) != 8 && len(shape.AudioComponent) > 2 {
		return fmt.Errorf("found %d audio components, expected 1, 2 or 16", len(shape.AudioComponent))
	}

	if len(shape.AudioComponent) == 0 {
		// We have no audio, so we fall back to silence
		streams := []int{0}
		for _, lang := range languagesToExport {
			clip.AudioFiles[lang] = &AudioFile{
				File:    EmptyWAVFile,
				Streams: streams,
			}
		}

		return nil
	}

	if len(shape.AudioComponent) == 1 {
		// We have stereo or mono audio, so we copy it to all languages
		for _, lang := range languagesToExport {
			clip.AudioFiles[lang] = &AudioFile{
				VXID:    clip.VXID,
				File:    shape.GetPath(),
				Streams: []int{shape.AudioComponent[0].EssenceStreamID},
			}
		}

		return nil
	}

	if len(shape.AudioComponent) == 2 {
		var streams []int
		for _, c := range shape.AudioComponent {
			streams = append(streams, c.EssenceStreamID)
			if c.ChannelCount != 1 {
				return fmt.Errorf("found %d channels in audio component, expected 1", c.ChannelCount)
			}
		}

		for _, lang := range languagesToExport {

			clip.AudioFiles[lang] = &AudioFile{
				VXID:    clip.VXID,
				File:    shape.GetPath(),
				Streams: streams,
			}
		}
	}

	if len(shape.AudioComponent) == 8 {
		if len(languagesToExport) > 1 {
			return fmt.Errorf("found 8 audio components, expected 16")
		}

		var streams []int
		for _, c := range shape.AudioComponent[:2] {
			streams = append(streams, c.EssenceStreamID)
			if c.ChannelCount != 1 {
				return fmt.Errorf("found %d channels in audio component, expected 1", c.ChannelCount)
			}
		}

		for _, lang := range languagesToExport {
			clip.AudioFiles[lang] = &AudioFile{
				VXID:    clip.VXID,
				File:    shape.GetPath(),
				Streams: streams,
			}
		}
	}

	for _, lang := range languagesToExport {
		// We have an actual 16 channel audio file, so we need to figure out which channels to use
		// and assign them to the correct language

		if l, ok := bccmflows.LanguagesByISO[lang]; ok {
			var streams []int
			for i := 0; i < l.MU1ChannelCount; i++ {
				streams = append(streams, l.MU1ChannelStart+i)
			}

			clip.AudioFiles[lang] = &AudioFile{
				VXID:    clip.VXID,
				File:    clip.VideoFile,
				Streams: streams,
			}
		} else if lang != "" {
			return errors.New("No language " + lang + " found in bccmflows.LanguagesByISO")
		}
	}

	return nil
}

// GetSubclipNames returns the names of all the subclips
func GetSubclipNames(client Client, itemVXID string) ([]string, error) {
	meta, err := client.GetMetadata(itemVXID)
	if err != nil {
		return nil, err
	}

	metaClips := lo.Values(meta.SplitByClips())

	sort.Slice(metaClips, func(i, j int) bool {
		inA, _, _ := metaClips[i].GetInOut(meta.Get(vscommon.FieldStartTC, "0@PAL"))
		inB, _, _ := metaClips[j].GetInOut(meta.Get(vscommon.FieldStartTC, "0@PAL"))
		return inA < inB
	})

	metaClips = lo.Filter(metaClips, func(i *vsapi.MetadataResult, _ int) bool {
		_, ok := i.Terse[vscommon.FieldStlText.Value]
		return !ok
	})

	keys := lo.Map(metaClips, func(i *vsapi.MetadataResult, _ int) string {
		return i.ID
	})

	return lo.Filter(keys, func(i string, _ int) bool {
		return i != vsapi.OriginalClip
	}), nil
}

// GetDataForExport returns the data needed to export the item with the given VXID
func GetDataForExport(client Client, itemVXID string, languagesToExport []string, audioSource *ExportAudioSource, subclip string, subsAllowAI bool) (*ExportData, error) {
	originalMeta, err := client.GetMetadata(itemVXID)
	if err != nil {
		return nil, err
	}

	metaClips := originalMeta.SplitByClips()

	// Get the metadata for the original clip
	meta := metaClips[vsapi.OriginalClip]

	originalLanguage := "no"
	for _, orLang := range originalMeta.Terse[vscommon.FieldLanguagesRecorded.Value] {
		if orLang.Value != "" {
			originalLanguage = orLang.Value
			break
		}
	}

	transcribedLanguage := originalLanguage
	for _, trLang := range originalMeta.Terse[vscommon.FieldTranscribedLanguage.Value] {
		if trLang.Value != "" {
			transcribedLanguage = trLang.Value
			break
		}
	}

	// Determine where to take the audio from
	if audioSource == nil {
		audioSource = &ExportAudioSourceEmbedded
		if meta.Get(vscommon.FieldExportAudioSource, "") == ExportAudioSourceRelated.Value {
			audioSource = &ExportAudioSourceRelated
		}
	}

	title := meta.Get(vscommon.FieldTitle, "")
	subclipTitle := subclip
	if subclipTitle != "" {
		title += " - " + subclipTitle
	}

	// clean up the title
	safeTitle := strings.ReplaceAll(title, " ", "_")
	safeTitle = nonAlphanumeric.ReplaceAllString(safeTitle, "")
	safeTitle = consecutiveUnderscores.ReplaceAllString(safeTitle, "_")

	var bmmTrackID *int
	if id := meta.Get(vscommon.FieldBmmTrackID, ""); id != "" {
		intID64, err := strconv.ParseInt(id, 10, 64)
		if err == nil {
			intID := int(intID64)
			bmmTrackID = &intID
		}
	}

	var bmmTitle *string
	if str := meta.Get(vscommon.FieldBmmTitle, ""); strings.TrimSpace(title) != "" {
		bmmTitle = &str
	}

	out := ExportData{
		Title:               title,
		SafeTitle:           safeTitle,
		Clips:               []*Clip{},
		BmmTrackID:          bmmTrackID,
		BmmTitle:            bmmTitle,
		OriginalLanguage:    originalLanguage,
		TranscribedLanguage: transcribedLanguage,
	}

	ingested := meta.Get(vscommon.FieldIngested, "")

	if ingested == "" {
		ingested = meta.Get(vscommon.FieldType{Value: "created"}, "")
	}

	if ingested != "" {
		t, err := time.Parse(time.RFC3339, ingested)
		if err == nil {
			out.ImportDate = &t
		}
	}

	// Get the video clips as a base
	out.Clips, err = ClipsFromMeta(client, itemVXID, originalMeta, subclipTitle)
	if err != nil {
		return nil, err
	}

	// Process the video clips and get the audio parts
	for _, clip := range out.Clips {
		clip.AudioFiles = map[string]*AudioFile{}

		if len(languagesToExport) == 0 {
			languagesToExport = meta.GetArray(vscommon.FieldLangsToExport)
		}

		if *audioSource == ExportAudioSourceRelated {
			err = enrichClipWithRelatedAudios(client, clip, languagesToExport)
		} else if *audioSource == ExportAudioSourceEmbedded {
			err = enrichClipWithEmbeddedAudio(client, clip, languagesToExport)
		}

		if err != nil {
			return nil, err
		}
	}

	err = addSubtitlesAndTranscriptionsToClips(client, out.Clips, subsAllowAI)
	if err != nil {
		return nil, err
	}

	return &out, nil
}

// addSubtitlesAndTranscriptionsToClips modifies the original clips to include subtitles and transcriptions
func addSubtitlesAndTranscriptionsToClips(client Client, clips []*Clip, allowAI bool) error {
	allSubLanguages := mapset.NewSet[string]()

	// Fetch subs
	for _, clip := range clips {
		clip.SubtitleFiles = map[string]string{}

		// This is independent of audio language export config, we include all subs available
		clipShapes, err := client.GetShapes(clip.VXID)
		if err != nil {
			return err
		}

		for langCode := range bccmflows.LanguagesByISO {
			// There are also videos with .txt subs... we should support those at some point
			shape := clipShapes.GetShape(fmt.Sprintf("sub_%s_srt", langCode))
			if shape == nil || shape.GetPath() == "" {
				continue
			}

			clip.SubtitleFiles[langCode] = shape.GetPath()

			// Collect all languages that any of the clips have subs for
			allSubLanguages.Add(langCode)
		}

		if len(clip.SubtitleFiles) == 0 && allowAI {
			// We have no subtitles, so we fall back to transcriptions
			shape := clipShapes.GetShape("Transcribed_Subtitle_SRT")
			if shape != nil && shape.GetPath() != "" {
				clip.SubtitleFiles["und"] = shape.GetPath()
			}
			allSubLanguages.Add("und")
		}

		shape := clipShapes.GetShape("transcription_json")
		if shape != nil {
			clip.JSONTranscriptFile = shape.GetPath()
		}
	}

	for _, clip := range clips {
		// Add empty subs for all languages that any of the clips have subs for if they are missing
		// This makes it easier to handle down the line if we always have a sub file for all languages
		for langCode := range allSubLanguages.Iter() {
			if _, ok := clip.SubtitleFiles[langCode]; !ok {
				clip.SubtitleFiles[langCode] = EmtpySRTFile
			}
		}
	}
	return nil
}

// convertFromClipTCTimeToSequenceRelativeTime ain't this a nice name?
func convertFromClipTCTimeToSequenceRelativeTime(clip *Clip, chapter *vsapi.MetadataResult, tcStart float64) *vsapi.MetadataResult {
	out := vsapi.MetadataResult{
		Terse: map[string][]*vsapi.MetadataField{},
	}

	// Claculate the offset from the clip position to the sequence position
	delta := clip.SequenceIn - clip.InSeconds

	for name, terseValue := range chapter.Terse {
		out.Terse[name] = terseValue

		for i, value := range chapter.Terse[name] {
			// Convert to seconds so we can use math
			chapterStart, _ := vscommon.TCToSeconds(value.Start)
			chapterEnd, _ := vscommon.TCToSeconds(value.End)

			// Convert to be relative to start of the media
			chapterStart = chapterStart - tcStart
			chapterEnd = chapterEnd - tcStart

			// Apply the offset to the chapter and convert back to TC
			out.Terse[name][i].Start = fmt.Sprintf("%.0f@PAL", (chapterStart+delta)*25)
			out.Terse[name][i].End = fmt.Sprintf("%.0f@PAL", (chapterEnd+delta)*25)
		}
	}

	return &out
}
