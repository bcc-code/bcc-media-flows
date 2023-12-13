package ingestworkflows

import (
	"sort"

	"github.com/bcc-code/bccm-flows/activities"
	"github.com/bcc-code/bccm-flows/paths"
	wfutils "github.com/bcc-code/bccm-flows/utils/workflows"
	"go.temporal.io/sdk/workflow"
)

type channelSource struct {
	Order   int
	Channel int
	Path    paths.Path
}

type channelSources []channelSource

func (s channelSources) Len() int {
	return len(s)
}

func (s channelSources) Less(i, j int) bool {
	return s[i].Order < s[j].Order
}

func (s channelSources) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

func Multitrack(ctx workflow.Context, params MasterParams) (*MasterResult, error) {
	logger := workflow.GetLogger(ctx)
	logger.Info("Starting Multitrack workflow")

	ctx = workflow.WithActivityOptions(ctx, wfutils.GetDefaultActivityOptions())

	tempDir, err := wfutils.GetWorkflowTempFolder(ctx)
	if err != nil {
		return nil, err
	}

	files, err := wfutils.ListFiles(ctx, params.Directory)
	if err != nil {
		return nil, err
	}

	sort.Sort(files)

	var channels paths.Files
	for _, f := range files {
		var parts paths.Files
		err = wfutils.ExecuteWithQueue(ctx, activities.SplitAudioChannels, activities.SplitAudioChannelsInput{
			FilePath:  f,
			OutputDir: tempDir,
		}).Get(ctx, &parts)
		if err != nil {
			return nil, err
		}
		channels = append(channels, parts...)
	}

	// make sure the files are sorted
	sort.Sort(channels)

	var muxResult activities.MultitrackMuxResult
	err = wfutils.ExecuteWithQueue(ctx, activities.MultitrackMux, activities.MultitrackMuxInput{
		Files:     channels,
		OutputDir: params.OutputDir,
	}).Get(ctx, &muxResult)
	if err != nil {
		return nil, err
	}

	base := files[0].Base()
	fileName := base[:len(base)-len(muxResult.OutputPath.Ext())]

	result, err := importFileAsTag(ctx, "original", muxResult.OutputPath, fileName)
	if err != nil {
		return nil, err
	}

	err = addMetaTags(ctx, result.AssetID, params.Metadata)
	if err != nil {
		return nil, err
	}

	err = wfutils.WaitForVidispineJob(ctx, result.ImportJobID)
	if err != nil {
		return nil, err
	}

	err = notifyImportCompleted(ctx, params.Targets, params.Metadata.JobProperty.JobID, map[string]paths.Path{
		result.AssetID: muxResult.OutputPath,
	})
	if err != nil {
		return nil, err
	}

	return nil, nil
}
