package transcode

import (
	"github.com/bcc-code/bccm-flows/common"
	"github.com/bcc-code/bccm-flows/utils"
	"os/exec"
	"path/filepath"
)

func AudioAac(input common.AudioInput, cb ProgressCallback) (*common.AudioResult, error) {
	params := []string{
		"-progress", "pipe:1",
		"-i", input.Path,
		"-c:a", "aac",
		"-b:a", input.Bitrate,
	}

	outputPath := filepath.Join(input.DestinationPath, filepath.Base(input.Path))

	//replace output extension to .aac
	outputPath = outputPath[:len(outputPath)-len(filepath.Ext(outputPath))] + ".aac"

	params = append(params, "-y", outputPath)

	info, err := ProbeFile(input.Path)
	if err != nil {
		return nil, err
	}

	cmd := exec.Command("ffmpeg", params...)

	_, err = utils.ExecuteCmd(cmd, parseProgressCallback(infoToBase(info), cb))
	if err != nil {
		return nil, err
	}
	return &common.AudioResult{
		OutputPath: outputPath,
	}, nil
}
