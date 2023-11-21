package scripts

import (
	"encoding/json"
	"fmt"
	"os"
	"path"
	"regexp"
	"time"

	"github.com/go-resty/resty/v2"
)

type BmmRelation struct {
	ID        int    `json:"id"`
	Type      string `json:"type"`
	Name      string `json:"name"`
	Timestamp int    `json:"timestamp"`
}

type BmmTrack struct {
	ID          int           `json:"id"`
	Title       string        `json:"title"`
	Copyright   string        `json:"copyright"`
	Language    string        `json:"language"`
	Rel         []BmmRelation `json:"rel"`
	Tags        []string      `json:"tags"`
	PublishedAt time.Time     `json:"published_at"`
	RecordedAt  time.Time     `json:"recorded_at"`
}

func GetBmmTracksFromVXItemsScript() {
	file := GetParam(2, "Enter file with vxitems: ")
	jsonFile, err := os.ReadFile(file)
	if err != nil {
		panic(err)
	}

	items := []*VxItem{}
	err = json.Unmarshal(jsonFile, &items)
	if err != nil {
		panic(err)
	}

	tracks, err := getBmmTracksFromVXItems(items)
	if err != nil {
		panic(err)
	}

	fmt.Printf("Found %d tracks\n", len(tracks))

	jsonTracks, err := json.Marshal(tracks)
	if err != nil {
		panic(err)
	}

	_ = os.Mkdir("output", 0755)
	err = os.WriteFile(path.Join("output", "tracks.json"), []byte(jsonTracks), 0644)
	if err != nil {
		panic(err)
	}
	fmt.Print("Saved to output/tracks.json\n")
}

func getBmmTracksFromVXItems(items []*VxItem) ([]*BmmTrack, error) {
	authToken := os.Getenv("SCRIPTS_BMM_TOKEN")

	if authToken == "" {
		panic("SCRIPTS_BMM_TOKEN env var is not set. Login and steal one from your browser at https://bmm-web.brunstad.org")
	}

	bmmClient := resty.New()
	bmmClient.Header.Set("Authorization", "Bearer "+authToken)
	bmmClient.Header.Set("Accept-Language", "nb")

	tracks := []*BmmTrack{}
	for _, item := range items {
		mbTitle := item.Title

		// Extract bmmId with regex from item.Title. title can for example be: BMM_JUL_100104_en. we want "100104" as bmmId
		regex := regexp.MustCompile(`BMM_.+_(\d+)_`)
		matches := regex.FindStringSubmatch(mbTitle)
		if len(matches) < 2 {
			return nil, fmt.Errorf("Could not extract bmmId from title: " + mbTitle)
		}
		bmmId := matches[1]

		result, err := bmmClient.R().SetResult(&BmmTrack{}).Get("https://bmm-api.brunstad.org/track/" + bmmId)
		if err != nil {
			return nil, err
		}
		fmt.Printf("Fetched metadata for track %s\n", bmmId)

		track := result.Result().(*BmmTrack)
		tracks = append(tracks, track)
	}
	return tracks, nil
}
