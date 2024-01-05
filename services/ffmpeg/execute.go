package ffmpeg

import (
	"os/exec"
	"strings"

	"github.com/bcc-code/bcc-media-flows/utils"
)

func Do(arguments []string, info StreamInfo, progressCallback ProgressCallback) (string, error) {
	cmd := exec.Command("ffmpeg", arguments...)

	if progressCallback != nil {
		progressCallback(Progress{
			Params: strings.Join(arguments, " "),
		})
	}

	return utils.ExecuteCmd(cmd, parseProgressCallback(arguments, info, progressCallback))
}
