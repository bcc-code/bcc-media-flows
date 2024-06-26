package activities

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	vsactivity "github.com/bcc-code/bcc-media-flows/activities/vidispine"
	"github.com/bcc-code/bcc-media-flows/paths"

	"github.com/bcc-code/bcc-media-flows/services/subtrans"
	"github.com/bcc-code/bcc-media-flows/services/vidispine"
	"github.com/bcc-code/bcc-media-flows/services/vidispine/vscommon"
	"go.temporal.io/sdk/temporal"
)

type GetSubtitlesInput struct {
	SubtransID        string
	Format            string
	ApprovedOnly      bool
	DestinationFolder paths.Path
	FilePrefix        string
}

type GetSubtransIDInput struct {
	VXID     string
	NoSubsOK bool
}

type GetSubtransIDOutput struct {
	SubtransID string
}

func (ua UtilActivities) GetSubtransIDActivity(_ context.Context, input GetSubtransIDInput) (*GetSubtransIDOutput, error) {
	out := &GetSubtransIDOutput{}

	vsClient := vsactivity.GetClient()
	subtransID, err := vidispine.GetSubtransID(vsClient, input.VXID)
	if err != nil {
		return out, err
	}

	if subtransID != "" {
		out.SubtransID = subtransID
		return out, nil
	}

	// We do not have a story ID saved, so we try to find it using the file name
	meta, err := vsClient.GetMetadata(input.VXID)
	if err != nil {
		return nil, err
	}

	originalUri := meta.Get(vscommon.FieldOriginalURI, "")

	parsedUri, err := url.Parse(originalUri)
	if err != nil {
		return out, err
	}

	// Extract file name
	fileName := filepath.Base(parsedUri.Path)

	// Split by dot
	fileNameSplit := strings.Split(fileName, ".")

	// Remove extension
	fileNameSplit = fileNameSplit[0 : len(fileNameSplit)-1]

	// Join back together
	fileName = strings.Join(fileNameSplit, ".")

	stClient := subtrans.NewClient(
		os.Getenv("SUBTRANS_BASE_URL"),
		os.Getenv("SUBTRANS_API_KEY"),
	)

	res, err := stClient.SearchByName(fileName)
	if err != nil {
		return out, err
	}
	if len(res) > 1 {
		return nil, temporal.NewNonRetryableApplicationError(fmt.Sprintf("multiple subtitles found for %s", fileName), "multiple_subtitles_found", nil)
	}

	if len(res) == 0 {
		if input.NoSubsOK {
			return out, nil
		}
		return nil, temporal.NewNonRetryableApplicationError(fmt.Sprintf("no subtitles found for %s", fileName), "multiple_subtitles_found", nil)
	}

	out.SubtransID = fmt.Sprintf("%d", res[0].ID)

	return out, nil
}

func (ua UtilActivities) GetSubtitlesActivity(_ context.Context, params GetSubtitlesInput) (map[string]paths.Path, error) {
	client := subtrans.NewClient(os.Getenv("SUBTRANS_BASE_URL"), os.Getenv("SUBTRANS_API_KEY"))

	info, err := os.Stat(params.DestinationFolder.Local())
	if os.IsNotExist(err) {
		return nil, err
	}

	if !info.IsDir() {
		return nil, fmt.Errorf("Destination path is not a directory")
	}

	subs, err := client.GetSubtitles(params.SubtransID, params.Format, params.ApprovedOnly)

	if err != nil {
		return nil, err
	}

	if params.FilePrefix == "" {
		p, _ := client.GetFilePrefix(params.SubtransID)
		params.FilePrefix = p
	}

	out := map[string]paths.Path{}
	for lang, sub := range subs {
		path := filepath.Join(params.DestinationFolder.Local(), params.FilePrefix+lang+"."+params.Format)
		err := os.WriteFile(path, []byte(sub), 0644)
		if err != nil {
			return nil, err
		}
		out[lang] = paths.MustParse(path)
	}
	return out, nil
}
