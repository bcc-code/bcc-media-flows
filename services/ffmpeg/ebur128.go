package ffmpeg

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strconv"

	"github.com/bcc-code/bccm-flows/utils"
)

type loudnormResult struct {
	InputIntegratedLoudness string `json:"input_i"`
	InputTruePeak           string `json:"input_tp"`
	InputLoudnessRange      string `json:"input_lra"`
	InputThreshold          string `json:"input_thresh"`
}

type AnalyzeEBUR128Result struct {
	InputIntegratedLoudness float64
	InputTruePeak           float64
	InputLoudnessRange      float64
	InputThreshold          float64
}

func floatOrZero(s string) float64 {
	f, _ := strconv.ParseFloat(s, 64)
	return f
}

func AnalyzeEBUR128(path string, progressCallback ProgressCallback) (*AnalyzeEBUR128Result, error) {
	cmd := exec.Command(
		"/opt/homebrew/bin/ffmpeg",
		"-hide_banner",
		"-nostats",
		//"-v", "quiet",
		"-i", path,
		"-af", "loudnorm=print_format=json",
		"-f", "null",
		"-",
	)

	info, err := GetStreamInfo(path)
	if err != nil {
		return nil, err
	}

	result, err := utils.ExecuteAnalysisCmd(cmd, parseProgressCallback(cmd.Args, info, progressCallback))
	if err != nil {
		return nil, fmt.Errorf("couldn't execute ffmpeg %s, %s, CMD: '%s'", path, err.Error(), cmd.String())
	}

	var analyzeResult loudnormResult
	err = json.Unmarshal([]byte(result), &analyzeResult)

	out := AnalyzeEBUR128Result{}
	out.InputIntegratedLoudness = floatOrZero(analyzeResult.InputIntegratedLoudness)
	out.InputTruePeak = floatOrZero(analyzeResult.InputTruePeak)
	out.InputLoudnessRange = floatOrZero(analyzeResult.InputLoudnessRange)
	out.InputThreshold = floatOrZero(analyzeResult.InputThreshold)

	return &out, err
}
