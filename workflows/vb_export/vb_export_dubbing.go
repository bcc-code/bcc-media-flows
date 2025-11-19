package vb_export

import (
	"fmt"

	bccmflows "github.com/bcc-code/bcc-media-flows"
	"github.com/bcc-code/bcc-media-flows/activities"
	vsactivity "github.com/bcc-code/bcc-media-flows/activities/vidispine"
	"github.com/bcc-code/bcc-media-flows/common"
	"github.com/bcc-code/bcc-media-flows/paths"
	"github.com/bcc-code/bcc-media-flows/services/rclone"
	"github.com/bcc-code/bcc-media-flows/services/vidispine/vscommon"
	wfutils "github.com/bcc-code/bcc-media-flows/utils/workflows"
	"go.temporal.io/sdk/workflow"
)

func VBExportToDubbing(ctx workflow.Context, params VBExportChildWorkflowParams) (*VBExportResult, error) {
	deliveryFolder := deliveryFolder.Append("Reaper-Wav", params.OriginalFilenameWithoutExt)

	logger := workflow.GetLogger(ctx)
	logger.Info("Starting ExportToAbekas")
	ctx = workflow.WithActivityOptions(ctx, wfutils.GetDefaultActivityOptions())

	dubbingOutputDir := params.TempDir.Append("dubbing_output")
	err := wfutils.CreateFolder(ctx, dubbingOutputDir)
	if err != nil {
		return nil, err
	}

	vxID := params.ParentParams.VXID

	relatedAudioFiles, err := wfutils.Execute(ctx, activities.Vidispine.GetRelatedAudioFiles, vxID).Result(ctx)
	if err != nil {
		return nil, err
	}

	if len(relatedAudioFiles) == 0 {
		return nil, fmt.Errorf("no related audio files found for VXID %s", vxID)
	}

	vxMeta, err := wfutils.Execute(ctx, activities.Vidispine.GetVXMetadata, vsactivity.VXOnlyParam{VXID: vxID}).Result(ctx)
	if err != nil {
		return nil, err
	}

	exportTC := vxMeta.Get(vscommon.FieldExportTCOverride, "00:00:00:00")

	transcodeSelector := workflow.NewSelector(ctx)

	langs, err := wfutils.GetMapKeysSafely(ctx, relatedAudioFiles)
	if err != nil {
		return nil, err
	}

	for _, lang := range langs {
		audioFile := relatedAudioFiles[lang]
		logger.Info("Starting audio transcoding", "lang", lang, "audioFile", audioFile)

		// Transcode audio file to WAV
		f := wfutils.Execute(ctx, activities.Audio.TranscodeToAudioWav, common.WavAudioInput{
			Path:            audioFile,
			DestinationPath: dubbingOutputDir,
			Timecode:        exportTC,
		})

		transcodeSelector.AddFuture(f.Future, postTranscodeAudio(ctx, audioFile, deliveryFolder, lang))
	}

	pilotFile := dubbingOutputDir.Append(params.OriginalFilenameWithoutExt + "_PILOT.wav")
	pilotResult, err := wfutils.Execute(ctx, activities.Audio.GenerateToneFile, activities.ToneInput{
		Duration:        params.AnalyzeResult.TotalSeconds,
		Frequency:       1000, // Fixed frequency for the pilot tone
		SampleRate:      48000,
		TimeCode:        exportTC,
		DestinationFile: pilotFile,
	}).Result(ctx)
	if err != nil {
		return nil, err
	}

	err = wfutils.RcloneCopyFile(ctx, pilotResult.OutputPath, deliveryFolder.Append(pilotResult.OutputPath.Base()), rclone.PriorityHigh)
	if err != nil {
		logger.Error("Error copying pilot file", "error", err)
		return nil, err
	}

	for i := 0; i < len(langs); i++ {
		transcodeSelector.Select(ctx)
	}

	return &VBExportResult{
		ID: vxID,
	}, nil
}

func postTranscodeAudio(ctx workflow.Context, originalFile paths.Path, destinationBase paths.Path, lang string) func(f workflow.Future) {
	logger := workflow.GetLogger(ctx)
	return func(f workflow.Future) {
		res := &common.AudioResult{}
		err := f.Get(ctx, res)
		if err != nil {
			logger.Error("Error transcoding audio", "error", err)
			return
		}

		//Naming: wav with trackNumber_languageCode at the end of the name e.g. BIST_S01_E07_MAS_NORmov_1_nor

		dubbReaperChannel := 99
		if l, ok := bccmflows.LanguagesByISO[lang]; ok {
			dubbReaperChannel = l.ReaperChannel
		}

		baseName := originalFile.BaseNoExt()
		dstName := res.OutputPath.Dir().Append(fmt.Sprintf("%s_%d_%s.wav", baseName, dubbReaperChannel, lang))

		wfutils.MoveFile(ctx, res.OutputPath, dstName, rclone.PriorityHigh)
		wfutils.RcloneCopyFile(ctx, dstName, destinationBase.Append(dstName.Base()), rclone.PriorityHigh)
	}
}
