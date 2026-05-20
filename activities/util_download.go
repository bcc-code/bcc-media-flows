package activities

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"github.com/bcc-code/bcc-media-flows/paths"
	"go.temporal.io/sdk/activity"
)

type DownloadFileFromURLInput struct {
	URL         string
	Destination paths.Path
}

type DownloadFileFromURLResult struct {
	Path paths.Path
	Size int64
}

func (ua UtilActivities) DownloadFileFromURL(ctx context.Context, in DownloadFileFromURLInput) (*DownloadFileFromURLResult, error) {
	log := activity.GetLogger(ctx)
	activity.RecordHeartbeat(ctx, "DownloadFileFromURL")
	log.Info("Starting DownloadFileFromURL", "url", in.URL, "destination", in.Destination.Local())

	stop := simpleHeartBeater(ctx)
	defer close(stop)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, in.URL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to build request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("download request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("download returned unexpected status: %s", resp.Status)
	}

	localPath := in.Destination.Local()
	if err := os.MkdirAll(filepath.Dir(localPath), os.ModePerm); err != nil {
		return nil, fmt.Errorf("failed to create destination directory: %w", err)
	}

	out, err := os.Create(localPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create destination file: %w", err)
	}

	written, err := io.Copy(out, resp.Body)
	closeErr := out.Close()
	if err != nil {
		return nil, fmt.Errorf("failed to write file: %w", err)
	}
	if closeErr != nil {
		return nil, fmt.Errorf("failed to close destination file: %w", closeErr)
	}

	_ = os.Chmod(localPath, os.ModePerm)

	return &DownloadFileFromURLResult{
		Path: in.Destination,
		Size: written,
	}, nil
}
