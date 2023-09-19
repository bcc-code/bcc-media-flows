package ffmpeg

import (
	"github.com/bcc-code/bccm-flows/utils"
	"os/exec"
)

func Do(arguments []string, info StreamInfo, progressCallback ProgressCallback) (string, error) {
	cmd := exec.Command("ffmpeg", arguments...)

	return utils.ExecuteCmd(cmd, parseProgressCallback(arguments, info, progressCallback))
}
