package scripts

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/davecgh/go-spew/spew"
)

type Episode struct {
	Label          string                           `json:"label"`
	Translations   DirectusCRUD[EpisodeTranslation] `json:"translations"`
	ContentType    string                           `json:"content_type"`
	Audience       string                           `json:"audience"`
	ProductionDate time.Time                        `json:"production_date"`
	Type           string                           `json:"type"`
	PublishDate    time.Time                        `json:"publish_date"`
}

type DirectusCRUD[T any] struct {
	Create []T `json:"create"`
	Update []T `json:"update"`
	Delete []T `json:"delete"`
}

type EpisodeTranslation struct {
	Title         string       `json:"title"`
	LanguagesCode LanguageCode `json:"languages_code"`
}

type LanguageCode struct {
	Code string `json:"code"`
}

func ImportBmmTracksAsVODEpisodesScript() {
	file := GetParam(2, "Enter file with tracks: ")
	jsonFile, err := os.ReadFile(file)
	if err != nil {
		panic(err)
	}

	tracks := []*BmmTrack{}
	err = json.Unmarshal(jsonFile, &tracks)
	if err != nil {
		panic(err)
	}

	err = importBmmTracksAsVODEpisodes(tracks)
	if err != nil {
		panic(err)
	}
}

func importBmmTracksAsVODEpisodes(tracks []*BmmTrack) error {
	for _, track := range tracks {
		err := importTrackAsEpisode(track)
		if err != nil {
			return err
		}
		fmt.Printf("Imported track %d: %s\n", track.ID, track.Title)
		return nil
	}
	return nil
}

func importTrackAsEpisode(track *BmmTrack) error {
	adminClient := requireAdminClient()

	// Create episode
	language := track.Language
	if language == "nb" {
		language = "no"
	}

	result, err := adminClient.R().SetBody(Episode{
		Label: fmt.Sprintf("BMM-%d: %s", track.ID, track.Title),
		Translations: DirectusCRUD[EpisodeTranslation]{
			Create: []EpisodeTranslation{
				{Title: track.Title, LanguagesCode: LanguageCode{Code: language}},
			},
			Update: []EpisodeTranslation{},
			Delete: []EpisodeTranslation{},
		},
		ContentType:    "other_transmission",
		Audience:       "general",
		ProductionDate: track.RecordedAt,
		Type:           "standalone",
		PublishDate:    track.PublishedAt,
	}).Post("/items/episodes")

	if err != nil {
		spew.Dump(result)
		return err
	}

	if strings.Contains(string(result.Body()), "errors") {
		spew.Dump(result)
		return fmt.Errorf("error creating episode")
	}

	return nil
}
