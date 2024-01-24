package ingestworkflows

import (
	"fmt"
	"strconv"
	"strings"

	bccmflows "github.com/bcc-code/bcc-media-flows"
	"github.com/bcc-code/bcc-media-flows/activities"
	vsactivity "github.com/bcc-code/bcc-media-flows/activities/vidispine"
	"github.com/bcc-code/bcc-media-flows/common"
	"github.com/bcc-code/bcc-media-flows/paths"
	wfutils "github.com/bcc-code/bcc-media-flows/utils/workflows"
	"go.temporal.io/sdk/workflow"
)

type IncrementalParams struct {
	Path string
}

// Incremental is a workflow that ingests a growing file into Vidispine.
// It also starts the Reaper recording.
//
// After the ingest is done, it stops the Reaper recording and adds the file to the placeholder.
// The reaper command returns the list of files that were recorded, so we can await for them to be
// available before padding them to the same start as the video file.
// The length of the files will typically be longer than video but that is not an issue.
//
// After the files are mdified, the need to be ingested into Vidispine, and
// linked properly to the video file
func Incremental(ctx workflow.Context, params IncrementalParams) error {
	logger := workflow.GetLogger(ctx)
	logger.Info("Starting Incremental")

	ctx = workflow.WithActivityOptions(ctx, wfutils.GetDefaultActivityOptions())

	in, err := paths.Parse(params.Path)
	if err != nil {
		return err
	}

	outDir, err := wfutils.GetWorkflowRawOutputFolder(ctx)
	if err != nil {
		return err
	}

	rawPath := outDir.Append(in.Base())

	/// Start file copy

	copyTask := wfutils.ExecuteWithQueue(ctx, activities.RsyncIncrementalCopy, activities.RsyncIncrementalCopyInput{
		In:  in,
		Out: rawPath,
	})

	var assetResult vsactivity.CreatePlaceholderResult
	err = wfutils.ExecuteWithQueue(ctx, vsactivity.CreatePlaceholderActivity, vsactivity.CreatePlaceholderParams{
		Title: in.Base(),
	}).Get(ctx, &assetResult)
	if err != nil {
		return err
	}
	wfutils.NotifyTelegramChannel(ctx, fmt.Sprintf("Starting live ingest: https://vault.bcc.media/item/%s", assetResult.AssetID))
	videoVXID := assetResult.AssetID

	// REAPER: Start recording
	err = wfutils.ExecuteWithQueue(ctx, activities.StartReaper, nil).Get(ctx, nil)
	if err != nil {
		return err
	}
	wfutils.NotifyTelegramChannel(ctx, "Reaper recording started")

	var jobResult vsactivity.FileJobResult
	err = wfutils.ExecuteWithQueue(ctx, vsactivity.AddFileToPlaceholder, vsactivity.AddFileToPlaceholderParams{
		AssetID:  videoVXID,
		FilePath: rawPath,
		Growing:  true,
	}).Get(ctx, &jobResult)
	if err != nil {
		return err
	}

	// Wait for file to be copied
	err = copyTask.Get(ctx, nil)
	if err != nil {
		return err
	}
	wfutils.NotifyTelegramChannel(ctx, fmt.Sprintf("Video ingest ended: https://vault.bcc.media/item/%s", assetResult.AssetID))

	// Stop Reaper recording
	reaperResult := &activities.StopReaperResult{}
	err = wfutils.ExecuteWithQueue(ctx, activities.StopReaper, nil).Get(ctx, reaperResult)
	if err != nil {
		return err
	}
	wfutils.NotifyTelegramChannel(ctx, fmt.Sprintf("Reaper recording stopped: %s", strings.Join(reaperResult.Files, ", ")))

	err = wfutils.ExecuteWithQueue(ctx, vsactivity.CloseFile, vsactivity.CloseFileParams{
		FileID: jobResult.FileID,
	}).Get(ctx, nil)
	if err != nil {
		return err
	}

	// Wait for all reaper files to be available
	waitForFileResult := map[paths.Path]workflow.Future{}
	for _, file := range reaperResult.Files {
		path := paths.MustParse(file)
		r := wfutils.ExecuteWithQueue(ctx, activities.WaitForFile, activities.FileInput{
			Path: path,
		})

		waitForFileResult[path] = r
	}

	outputFolder, err := wfutils.GetWorkflowRawOutputFolder(ctx)
	if err != nil {
		return err
	}

	addSilenceFutures := map[paths.Path]workflow.Future{}

	baseFileName := strings.TrimSuffix(rawPath.Base(), "_MU1.mxf")

	// As Reaper files become available, prepend silence to them
	for len(waitForFileResult) > 0 {
		newWaitForFileResult := map[paths.Path]workflow.Future{}
		for path, res := range waitForFileResult {
			if !res.IsReady() {
				newWaitForFileResult[path] = res
				continue
			}

			fileOK := false
			err := res.Get(ctx, &fileOK)
			if err != nil {
				return err
			}

			isSilent := false
			err = wfutils.ExecuteWithQueue(ctx, activities.DetectSilence, common.AudioInput{
				Path: path,
			}).Get(ctx, &isSilent)
			if err != nil {
				return err
			}

			if isSilent {
				wfutils.NotifyTelegramChannel(ctx, fmt.Sprintf("File %s is silent, skipping", path.Base()))
				continue
			}

			// ReaperTrack-DATE_TIME.wav
			// 22-240122_1526.wav
			reaperTrackNumber, err := strconv.Atoi(strings.Split(path.Base(), ".")[0])
			if err != nil {
				return err
			}

			// Generate a filename withe the language code
			outPath := outputFolder.Append(fmt.Sprintf("%s-%s.wav", baseFileName, strings.ToUpper(bccmflows.LanguagesByReaper[reaperTrackNumber].ISO6391)))
			addSilenceFuture := wfutils.ExecuteWithQueue(ctx, activities.PrependSilence, activities.PrependSilenceInput{
				FilePath: path,
				Output:   outPath,
			})
			addSilenceFutures[outPath] = addSilenceFuture
		}
		waitForFileResult = newWaitForFileResult
	}

	for k, f := range addSilenceFutures {
		err := f.Get(ctx, nil)
		if err != nil {
			return err
		}

		var assetResult vsactivity.CreatePlaceholderResult
		err = wfutils.ExecuteWithQueue(ctx, vsactivity.CreatePlaceholderActivity, vsactivity.CreatePlaceholderParams{
			Title: k.Base(),
		}).Get(ctx, &assetResult)
		if err != nil {
			return err
		}

		err = wfutils.ExecuteWithQueue(ctx, vsactivity.AddFileToPlaceholder, vsactivity.AddFileToPlaceholderParams{
			FilePath: k,
			AssetID:  assetResult.AssetID,
			Growing:  false,
		}).Get(ctx, nil)
		if err != nil {
			return err
		}

		err = wfutils.ExecuteWithQueue(ctx, vsactivity.AddRelation, vsactivity.AddRelationParams{
			Child:  assetResult.AssetID,
			Parent: videoVXID,
		}).Get(ctx, nil)
		if err != nil {
			return err
		}

	}

	return nil
}
