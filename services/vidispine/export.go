package vidispine

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/bcc-code/bccm-flows/environment"

	bccmflows "github.com/bcc-code/bccm-flows"
	"github.com/bcc-code/bccm-flows/services/vidispine/vsapi"
	"github.com/bcc-code/bccm-flows/services/vidispine/vscommon"
	mapset "github.com/deckarep/golang-set/v2"
	"github.com/orsinium-labs/enum"
	"github.com/samber/lo"
)

var nonAlphanumeric = regexp.MustCompile("[^a-zA-Z0-9_]+")
var consecutiveUnderscores = regexp.MustCompile("_+")

type Clip struct {
	VideoFile     string
	InSeconds     float64
	OutSeconds    float64
	SequenceIn    float64
	SequenceOut   float64
	AudioFiles    map[string]*AudioFile
	SubtitleFiles map[string]string
	VXID          string
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

func (s *VidispineService) getClipForAssetOrSubclip(
	itemVXID string,
	isSubclip bool,
	meta *vsapi.MetadataResult,
	clipsMeta map[string]*vsapi.MetadataResult,
) (*Clip, error) {
	shapes, err := s.apiClient.GetShapes(itemVXID)
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

	var subclipMeta *vsapi.MetadataResult

	subclipName := meta.Get(vscommon.FieldSubclipToExport, "")
	if scMeta, ok := clipsMeta[subclipName]; ok {
		subclipMeta = scMeta
	} else {
		return nil, errors.New("Subclip " + subclipName + " does not exist")
	}

	in, out, err := subclipMeta.GetInOut(meta.Get(vscommon.FieldStartTC, "0@PAL"))
	clip.InSeconds = in
	clip.OutSeconds = out
	return &clip, err
}

func (s *VidispineService) getRelatedAudios(clip *Clip, oLanguagesToExport []string) (*Clip, error) {

	languagesToExport := make([]string, len(oLanguagesToExport))
	copy(languagesToExport, oLanguagesToExport)

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
		clipMeta, err := s.apiClient.GetMetadata(clip.VXID)
		if err != nil {
			return clip, err
		}

		// Now we know what audio to export
		relatedAudioVXID := clipMeta.Get(vscommon.FieldType{relatedField}, "")
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

		relatedAudioShapes, err := s.apiClient.GetShapes(relatedAudioVXID)
		if err != nil {
			return clip, err
		}

		// Ok now we can finally get the path to the audio file
		relatedAudioShape := relatedAudioShapes.GetShape("original")
		if relatedAudioShape == nil {
			if languagesToExport[0] == "nor" {
				// Fall back to "nor" audio and issue a warning *somewhere*
				clip.AudioFiles[lang] = clip.AudioFiles["nor"]
			} else {
				return clip, fmt.Errorf("no original or fallback shape found for item %s", relatedAudioVXID)
			}
			continue
		}

		var streams []int

		if len(relatedAudioShape.AudioComponent) > 0 {
			streams = append(streams, relatedAudioShape.AudioComponent[0].EssenceStreamID)
		} else {
			return clip, fmt.Errorf("no audio components found for item %s", relatedAudioVXID)
		}

		clip.AudioFiles[lang] = &AudioFile{
			VXID:    relatedAudioVXID,
			File:    relatedAudioShape.GetPath(),
			Streams: streams,
		}
	}

	return clip, nil
}

func (s *VidispineService) getEmbeddedAudio(clip *Clip, languagesToExport []string) (*Clip, error) {
	shapes, err := s.apiClient.GetShapes(clip.VXID)
	if err != nil {
		return clip, err
	}

	shape := shapes.GetShape("original")
	if len(shape.AudioComponent) != 16 && len(shape.AudioComponent) > 2 {
		return clip, fmt.Errorf("found %d audio components, expected 1, 2 or 16", len(shape.AudioComponent))
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

		return clip, nil
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

		return clip, nil
	}

	if len(shape.AudioComponent) == 2 {
		var streams []int
		for _, c := range shape.AudioComponent {
			streams = append(streams, c.EssenceStreamID)
			if c.ChannelCount != 1 {
				return clip, fmt.Errorf("found %d channels in audio component, expected 1", c.ChannelCount)
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
		} else {
			return clip, errors.New("No language " + lang + " found in bccmflows.LanguagesByISO")
		}
	}

	return clip, nil
}

// GetDataForExport returns the data needed to export the item with the given VXID
// If exportSubclip is true, the subclip will be exported, otherwise the whole clip
func (s *VidispineService) GetDataForExport(itemVXID string, languagesToExport []string, audioSource *ExportAudioSource) (*ExportData, error) {
	meta, err := s.apiClient.GetMetadata(itemVXID)
	if err != nil {
		return nil, err
	}

	// Check for subclip
	// This check needs to happen on the original metadata, not the split one
	isSubclip := len(meta.GetArray(vscommon.FieldTitle)) > 1

	metaClips := meta.SplitByClips()

	// Get the metadata for the original clip
	meta = metaClips[vsapi.OriginalClip]

	// Determine where to take the audio from
	if audioSource == nil {
		audioSource = &ExportAudioSourceEmbedded
		if meta.Get(vscommon.FieldExportAudioSource, "") == ExportAudioSourceRelated.Value {
			audioSource = &ExportAudioSourceRelated
		}
	}

	// Check for sequence
	isSequence := meta.Get(vscommon.FieldSequenceSize, "0") != "0"

	title := meta.Get(vscommon.FieldSubclipToExport, meta.Get(vscommon.FieldTitle, ""))

	// clean up the title
	safeTitle := strings.ReplaceAll(title, " ", "_")
	safeTitle = nonAlphanumeric.ReplaceAllString(safeTitle, "")
	safeTitle = consecutiveUnderscores.ReplaceAllString(safeTitle, "_")

	out := ExportData{
		Title:     title,
		SafeTitle: safeTitle,
		Clips:     []*Clip{},
	}

	// Get the video clips as a base
	if isSequence {
		seq, err := s.apiClient.GetSequence(itemVXID)
		if err != nil {
			return nil, err
		}
		out.Clips, err = s.SeqToClips(seq, *audioSource)
		if err != nil {
			return nil, err
		}
	} else {
		clip, err := s.getClipForAssetOrSubclip(itemVXID, isSubclip, meta, metaClips)
		if err != nil {
			return nil, err
		}
		out.Clips = append(out.Clips, clip)
	}

	// Process the video clips and get the audio parts
	for _, clip := range out.Clips {
		clip.AudioFiles = map[string]*AudioFile{}

		if len(languagesToExport) == 0 {
			languagesToExport = meta.GetArray(vscommon.FieldLangsToExport)
		}

		if *audioSource == ExportAudioSourceRelated {
			clip, err = s.getRelatedAudios(clip, languagesToExport)
		} else if *audioSource == ExportAudioSourceEmbedded {
			clip, err = s.getEmbeddedAudio(clip, languagesToExport)
		}

		if err != nil {
			return nil, err
		}
	}

	allSubLanguages := mapset.NewSet[string]()

	// Fetch subs
	for _, clip := range out.Clips {
		clip.SubtitleFiles = map[string]string{}

		// This is independent of audio language export config, we include all subs available
		clipShapes, err := s.apiClient.GetShapes(clip.VXID)
		if err != nil {
			return nil, err
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
	}

	for _, clip := range out.Clips {
		// Add empty subs for all languages that any of the clips have subs for if they are missing
		// This makes it easier to handle down the line if we always have a sub file for all languages
		for langCode := range allSubLanguages.Iter() {
			if _, ok := clip.SubtitleFiles[langCode]; !ok {
				clip.SubtitleFiles[langCode] = EmtpySRTFile
			}
		}
	}

	return &out, nil
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
