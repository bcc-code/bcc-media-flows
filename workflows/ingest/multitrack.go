package ingestworkflows

import (
	"sort"
	"strconv"
	"strings"

	"github.com/bcc-code/bccm-flows/activities"
	"github.com/bcc-code/bccm-flows/paths"
	"github.com/bcc-code/bccm-flows/services/transcode"
	wfutils "github.com/bcc-code/bccm-flows/utils/workflows"
	"github.com/samber/lo"
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
		err = wfutils.ExecuteWithQueue(ctx, activities.AudioSplitFiles, transcode.AudioSplitFileInput{
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

func filesToChannelSources(files []paths.Path) (channelSources, error) {
	var sources channelSources
	for _, f := range files {
		r, err := getChannelSourcesFromFile(f)
		if err != nil {
			return nil, err
		}
		sources = append(sources, r...)
	}

	sort.Sort(sources)
	return sources, nil
}

func getChannelSourcesFromFile(file paths.Path) ([]channelSource, error) {
	base := file.Base()
	name := base[:len(base)-len(file.Ext())]
	parts := strings.Split(name, "_")
	last, _ := lo.Last(parts)
	if last == "" {
		return nil, nil
	}

	channel1, err := strconv.Atoi(string(last[0]))
	if err != nil {
		return nil, err
	}
	sources := []channelSource{
		{
			Order:   channel1,
			Channel: 0,
			Path:    file,
		},
	}
	if len(last) == 1 {
		return sources, nil
	}
	channel2, err := strconv.Atoi(string(last[1]))
	if err != nil {
		return nil, err
	}
	return append(sources, channelSource{
		Order:   channel2,
		Channel: 1,
		Path:    file,
	}), nil
}
