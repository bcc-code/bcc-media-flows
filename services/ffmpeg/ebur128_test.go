package ffmpeg

import (
	"encoding/json"
	"github.com/bcc-code/bcc-media-flows/utils"
	"github.com/stretchr/testify/assert"
	"os/exec"
	"testing"
)

func TestExecuteAnalysisCmd_Normalize(t *testing.T) {
	cmd := exec.Command(
		"ffmpeg",
		"-hide_banner",
		"-nostats",
		"-i", "./test_files/file_example_WAV_1MG.wav",
		"-af", "loudnorm=print_format=json",
		"-f", "null",
		"-progress", "pipe:1",
		"-",
	)

	callback := func(s string) {
		t.Log(s)
	}

	res, err := utils.ExecuteAnalysisCmd(cmd, callback)

	assert.NoError(t, err)
	assert.NotEmpty(t, res)

	var analyzeResult loudnormResult
	err = json.Unmarshal([]byte(res), &analyzeResult)
	assert.NoError(t, err)
	assert.Equal(t, loudnormResult{
		InputIntegratedLoudness: "-20.60",
		InputTruePeak:           "-14.66",
		InputLoudnessRange:      "3.50",
	}, analyzeResult)
}
