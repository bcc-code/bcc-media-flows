package ingestworkflows

import (
	"fmt"
	"strings"
	"time"

	bccmflows "github.com/bcc-code/bcc-media-flows"
	"github.com/bcc-code/bcc-media-flows/activities"
	vsactivity "github.com/bcc-code/bcc-media-flows/activities/vidispine"
	"github.com/bcc-code/bcc-media-flows/paths"
	wfutils "github.com/bcc-code/bcc-media-flows/utils/workflows"
	"go.temporal.io/sdk/workflow"
)

type ExtractAudioFromMU1MU2Input struct {
	MU1ID string
	MU2ID string
}

func ExtractAudioFromMU1MU2(ctx workflow.Context, input ExtractAudioFromMU1MU2Input) error {
	logger := workflow.GetLogger(ctx)
	logger.Info("Starting ExtractAudioFromMU1MU2 workflow")

	ctx = workflow.WithActivityOptions(ctx, wfutils.GetDefaultActivityOptions())

	// Get paths to the original files
	MU1FileFuture := wfutils.Execute(ctx, vsactivity.GetFileFromVXActivity, vsactivity.GetFileFromVXParams{
		VXID: input.MU1ID,
		Tags: []string{"original"},
	})

	MU2FileFuture := wfutils.Execute(ctx, vsactivity.GetFileFromVXActivity, vsactivity.GetFileFromVXParams{
		VXID: input.MU2ID,
		Tags: []string{"original"},
	})

	Mu1Result := &vsactivity.GetFileFromVXResult{}
	Mu2Result := &vsactivity.GetFileFromVXResult{}
	err := MU1FileFuture.Get(ctx, Mu1Result)
	if err != nil {
		return err
	}
	err = MU2FileFuture.Get(ctx, Mu2Result)
	if err != nil {
		return err
	}

	// Calculte TC difference between MU1 and MU2
	sampleOffset := int(0)
	err = wfutils.Execute(ctx, activities.GetVideoOffset, activities.GetVideoOffsetInput{
		VideoPath1:      Mu1Result.FilePath,
		VideoPath2:      Mu2Result.FilePath,
		VideoFPS:        25,
		AudioSampleRate: 48000,
	}).Get(ctx, &sampleOffset)
	if err != nil {
		return err
	}

	outputPath, err := wfutils.GetWorkflowTempFolder(ctx)
	if err != nil {
		return err
	}

	baseFileName := strings.TrimSuffix(Mu1Result.FilePath.Base(), "_MU1.mxf")

	// Extract audio from MU1
	extract1Future := wfutils.Execute(ctx, activities.ExtractAudio, activities.ExtractAudioInput{
		VideoPath:       Mu1Result.FilePath,
		OutputFolder:    outputPath,
		FileNamePattern: baseFileName + "_MU1CH_%d.wav",
	})

	// Extract audio from MU2
	extract2Future := wfutils.Execute(ctx, activities.ExtractAudio, activities.ExtractAudioInput{
		VideoPath:       Mu2Result.FilePath,
		OutputFolder:    outputPath,
		FileNamePattern: baseFileName + "_MU2CH_%d.wav",
	})

	// Wait for both audio extractions to finish
	mu1Files, err := extract1Future.Result(ctx)
	if err != nil {
		return err
	}

	mu2Files, err := extract2Future.Result(ctx)
	if err != nil {
		return err
	}

	destinationPath, err := wfutils.GetWorkflowRawOutputFolder(ctx)
	if err != nil {
		return err
	}

	filesToImport := map[string]paths.Path{}
	var futures []workflow.Future

	// Align audio from MU1 and MU2

	keys, err := wfutils.GetMapKeysSafely(ctx, mu2Files.AudioFiles)
	if err != nil {
		return err
	}

	if sampleOffset < 0 {
		for _, key := range keys {
			file := mu2Files.AudioFiles[key]
			outputFile := destinationPath.Append(file.Base())
			f := wfutils.Execute(ctx, activities.TrimFile, activities.TrimInput{
				Input:  file,
				Output: outputFile,
				Start:  float64(-sampleOffset) / float64(48000),
			})

			futures = append(futures, f.Future)
			filesToImport[bccmflows.LanguagesByMU2[key].ISO6391] = outputFile
		}
	} else if sampleOffset > 0 {
		for _, key := range keys {
			file := mu2Files.AudioFiles[key]
			outputFile := destinationPath.Append(file.Base())
			f := wfutils.Execute(ctx, activities.PrependSilence, activities.PrependSilenceInput{
				FilePath:   file,
				Output:     outputFile,
				SampleRate: 48000,
				Samples:    sampleOffset,
			})

			futures = append(futures, f.Future)
			filesToImport[bccmflows.LanguagesByMU2[key].ISO6391] = outputFile
		}
	} else {
		return fmt.Errorf("no offset - this is extremely unlikely to happen, please check the input files - STOPPING WORKFLOW")
	}

	// We do not touch MU1 audio files
	for i, file := range mu1Files.AudioFiles {
		destinationFile := destinationPath.Append(file.Base())
		f := wfutils.Execute(ctx, activities.CopyFile, activities.MoveFileInput{
			Source:      file,
			Destination: destinationFile,
		})
		futures = append(futures, f.Future)
		filesToImport[bccmflows.LanguagesByMU1[i].ISO6391] = destinationFile
	}

	errors := ""
	for _, f := range futures {
		err = f.Get(ctx, nil)
		if err != nil {
			errors += err.Error() + "\n"
		}
	}

	if errors != "" {
		return fmt.Errorf("errors while aligning audio: %s", errors)
	}

	// Import to MB
	err = RelateAudioToVideo(ctx, RelateAudioToVideoParams{
		VideoVXID:    input.MU1ID,
		AudioList:    filesToImport,
		PreviewDelay: 1 * time.Second,
	})

	return err
}
