package bmm

import (
	"fmt"
	"strconv"
	"strings"

	ingestworkflows "github.com/bcc-code/bcc-media-flows/workflows/ingest"
)

type MappingOptions struct {
	// Language overrides which Translations entry is used. Empty = OriginalLanguage.
	Language string
	// FileBaseURL is prefixed to the Path of the selected audio file. Empty = leave fileUrl empty.
	FileBaseURL string
	// VXSourceOverride replaces the value taken from MediabankenId. Empty = no override.
	VXSourceOverride string
	// FileURLOverride replaces the computed file URL. Empty = use the computed value.
	FileURLOverride string
	// CuratedPlaylist, when set, records that this track is being imported as part of a
	// curated playlist; the value is forwarded verbatim into BmmTrackMetadataParams so it
	// lands in the asset's metadata JSON. Nil = no playlist provenance.
	CuratedPlaylist *ingestworkflows.BmmCuratedPlaylist
}

// ToWorkflowParams converts a RavenTrack into the BmmTrackMetadataParams expected by the workflow.
// All field-name knowledge between the two schemas lives here.
func ToWorkflowParams(t *RavenTrack, opts MappingOptions) (ingestworkflows.BmmTrackMetadataParams, error) {
	lang := opts.Language
	if lang == "" {
		lang = t.OriginalLanguage
	}

	tr := findTranslation(t.Translations, lang)
	if tr == nil {
		return ingestworkflows.BmmTrackMetadataParams{}, fmt.Errorf("track %d has no translation for language %q", t.ID, lang)
	}

	songNumbers := make([]string, 0)
	contributors := make([]ingestworkflows.BmmContributor, 0)
	for _, rel := range t.Rel {
		switch rel.Type {
		case "songbook":
			songNumbers = append(songNumbers, fmt.Sprintf("%s-%d", songbookPrefix(rel.Name), rel.ID))
		case "external", "":
			// Skip external links and untyped rels.
		default:
			contributors = append(contributors, ingestworkflows.BmmContributor{
				ID:   strconv.Itoa(rel.ID),
				Name: rel.Name,
				Role: rel.Type,
			})
		}
	}

	fileURL := opts.FileURLOverride
	if fileURL == "" {
		path := pickAudioFilePath(tr.Media)
		if path != "" && opts.FileBaseURL != "" {
			fileURL = strings.TrimRight(opts.FileBaseURL, "/") + "/" + strings.TrimLeft(path, "/")
		}
	}

	vxSource := opts.VXSourceOverride
	if vxSource == "" {
		vxSource = t.MediabankenID
	}

	title := tr.Title
	if title == "" {
		title = tr.Meta.Title
	}

	tags := t.Tags
	if tags == nil {
		tags = []string{}
	}

	return ingestworkflows.BmmTrackMetadataParams{
		BmmTrackID:    t.ID,
		SongNumbers:   songNumbers,
		Contributors:  contributors,
		Title:         title,
		Language:      tr.Meta.Language,
		PublishedDate: t.PublishedAt,
		RecordedAt:    t.RecordedAt,
		Copyright:     tr.Meta.Copyright,
		Album: ingestworkflows.BmmAlbum{
			ID:   strconv.Itoa(t.ParentID),
			Name: tr.Meta.Album,
		},
		CuratedPlaylist: opts.CuratedPlaylist,
		Tags:            tags,
		VXSource:        vxSource,
		FileURL:         fileURL,
	}, nil
}

func findTranslation(translations []RavenTranslation, language string) *RavenTranslation {
	for i := range translations {
		if translations[i].Language == language {
			return &translations[i]
		}
	}
	return nil
}

// songbookPrefixes is the BMM-convention abbreviation per known songbook slug.
// Only the two legacy songbooks use lowercase slugs as rel names; newer ones
// (NHV, NFMB, RB, SOS — see availableSongbooks in the BMM-Website admin) store
// the uppercase abbreviation directly and are handled by the uppercase
// pass-through in songbookPrefix. Slugs not covered by either fall through to
// the initials heuristic.
var songbookPrefixes = map[string]string{
	"herrens_veier":  "HV",
	"mandelblomsten": "FMB",
}

// songbookPrefix returns the conventional abbreviation for a songbook slug
// (e.g. "herrens_veier" -> "HV"). Unknown slugs fall back to: if already
// uppercase, pass-through; otherwise take the first letter of each
// underscore-separated word and uppercase it.
func songbookPrefix(slug string) string {
	if p, ok := songbookPrefixes[slug]; ok {
		return p
	}
	if slug == strings.ToUpper(slug) && slug != "" {
		return slug
	}
	parts := strings.Split(slug, "_")
	out := make([]byte, 0, len(parts))
	for _, p := range parts {
		if p == "" {
			continue
		}
		out = append(out, p[0])
	}
	return strings.ToUpper(string(out))
}

// pickAudioFilePath returns the audio file with the largest Size, on the assumption
// that the largest file is the best-quality / full-length original.
func pickAudioFilePath(groups []RavenMediaGroup) string {
	var best RavenMediaFile
	for _, g := range groups {
		if g.Type != "audio" {
			continue
		}
		for _, f := range g.Files {
			if f.Size > best.Size {
				best = f
			}
		}
	}
	return best.Path
}
