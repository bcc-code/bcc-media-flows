package activities

import (
	"context"
	"errors"

	"github.com/bcc-code/bcc-media-flows/paths"
	"github.com/bcc-code/bcc-media-flows/services/fcplibrary"
	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/temporal"
)

type ExtractFCPLibraryInput struct {
	// Archive is the .fcpbundlezip Cantemo Portal stores the library as.
	Archive paths.Path
	// Destination is the folder the library is extracted into.
	Destination paths.Path
}

type ExtractFCPLibraryResult struct {
	Library paths.Path
	// Items are the media items to place in the library, with paths relative to it.
	Items   []fcplibrary.Item
	Skipped []fcplibrary.SkippedItem
}

// ExtractFCPLibrary unzips a Cantemo Portal FCP library into Destination and returns
// the media items its portalproject.xml says belong in it.
func (ua UtilActivities) ExtractFCPLibrary(ctx context.Context, input ExtractFCPLibraryInput) (*ExtractFCPLibraryResult, error) {
	log := activity.GetLogger(ctx)
	log.Info("Starting ExtractFCPLibrary", "archive", input.Archive.Linux())

	stop := simpleHeartBeater(ctx)
	defer close(stop)

	library := input.Destination.Append(fcplibrary.LibraryName(input.Archive.Base()))
	if err := fcplibrary.Extract(input.Archive.Local(), library.Local()); err != nil {
		return nil, err
	}

	items, skipped, err := fcplibrary.Items(library.Local())
	if errors.Is(err, fcplibrary.ErrNoPortalProject) {
		return nil, temporal.NewNonRetryableApplicationError(err.Error(), "NoPortalProject", err)
	}
	if err != nil {
		return nil, err
	}

	return &ExtractFCPLibraryResult{Library: library, Items: items, Skipped: skipped}, nil
}
