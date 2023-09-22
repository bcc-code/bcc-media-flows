package activities

import (
	"context"
	"fmt"
	"os"
	"path"

	"github.com/bcc-code/bccm-flows/services/subtrans"
)

type GetSubtitlesInput struct {
	SubtransID        string
	Format            string
	ApprovedOnly      bool
	DestinationFolder string
	FilePrefix        string
}

func SubtransGetSubtitles(ctx context.Context, params GetSubtitlesInput) (map[string]string, error) {
	client := subtrans.NewClient(os.Getenv("SUBTRANS_BASE_URL"), os.Getenv("SUBTRANS_API_KEY"))

	info, err := os.Stat(params.DestinationFolder)
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
	out := map[string]string{}
	for lang, sub := range subs {
		path := path.Join(params.DestinationFolder, params.FilePrefix+lang+"."+params.Format)
		err := os.WriteFile(path, []byte(sub), 0644)
		if err != nil {
			return nil, err
		}
		out[lang] = path

	}
	return out, nil
}
