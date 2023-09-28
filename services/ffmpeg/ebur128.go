package ffmpeg

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strconv"

	"github.com/bcc-code/bccm-flows/common"
	"github.com/bcc-code/bccm-flows/utils"
)

type loudnormResult struct {
	InputIntegratedLoudness string `json:"input_i"`
	InputTruePeak           string `json:"input_tp"`
	InputLoudnessRange      string `json:"input_lra"`
}

func floatOrZero(s string) float64 {
	f, _ := strconv.ParseFloat(s, 64)
	return f
}

func AnalyzeEBUR128(path string, progressCallback ProgressCallback) (*common.AnalyzeEBUR128Result, error) {
	cmd := exec.Command(
		"/opt/homebrew/bin/ffmpeg",
		"-hide_banner",
		"-nostats",
		//"-v", "quiet",
		"-i", path,
		"-af", "loudnorm=print_format=json",
		"-f", "null",
		"-progress", "pipe:1",
		"-",
	)

	info, err := GetStreamInfo(path)
	if err != nil {
		return nil, err
	}

	result, err := utils.ExecuteAnalysisCmd(cmd, parseProgressCallback(cmd.Args, info, progressCallback))
	if err != nil {
		return nil, fmt.Errorf("couldn't execute ffmpeg %s, %w, CMD: '%s'", path, err, cmd.String())
	}

	var analyzeResult loudnormResult
	err = json.Unmarshal([]byte(result), &analyzeResult)
	if err != nil {
		return nil, fmt.Errorf("couldn't parse ffmpeg output %s, %w, CMD: '%s', Res: \n---------\n%s\n--------------\n", path, err, cmd.String(), result)
	}

	out := common.AnalyzeEBUR128Result{}
	out.IntegratedLoudness = floatOrZero(analyzeResult.InputIntegratedLoudness)
	out.TruePeak = floatOrZero(analyzeResult.InputTruePeak)
	out.LoudnessRange = floatOrZero(analyzeResult.InputLoudnessRange)

	return &out, err
}
