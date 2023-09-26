package workflows

import (
	"encoding/xml"
	"github.com/bcc-code/bccm-flows/services/ingest"
	"go.temporal.io/sdk/workflow"
	"io"
	"os"
)

type AssetIngestParams struct{}

type AssetIngestResult struct{}

func AssetIngest(ctx workflow.Context, params AssetIngestParams) (*AssetIngestResult, error) {
	var path string

	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	contents, err := io.ReadAll(file)
	if err != nil {
		return nil, err
	}

	var metadata ingest.Metadata
	err = xml.Unmarshal(contents, &metadata)
	if err != nil {
		return nil, err
	}

	for _, f := range metadata.FileList.Files {
		if f.IsFolder {
			continue
		}

	}

	return &AssetIngestResult{}, nil
}
