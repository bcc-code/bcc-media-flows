package rsync

import (
	"os"
	"os/exec"
	"time"

	"github.com/bcc-code/bcc-media-flows/paths"
	"github.com/bcc-code/bcc-media-flows/utils"
)

type FileInfo struct {
	Size    int64
	Name    string
	ModTime time.Time
}

func IncrementalCopy(in, out paths.Path, statCallback func(FileInfo)) error {
	var fileSize int64
	doCopy := func() (bool, error) {
		params := []string{
			"-a",
			"--progress",
			"--inplace",
			"--append",
			in.Local(),
			out.Local(),
		}

		cmd := exec.Command("rsync", params...)

		_, err := utils.ExecuteCmd(cmd, nil)
		if err != nil {
			return false, err
		}

		info, err := os.Stat(out.Local())
		if err != nil {
			return false, err
		}
		if info.Size() >= 10 && info.Size() == fileSize {
			return true, nil
		}

		fileSize = info.Size()
		statCallback(FileInfo{
			Name:    info.Name(),
			Size:    info.Size(),
			ModTime: info.ModTime(),
		})
		return false, nil
	}

	for {
		done, err := doCopy()
		if err != nil {
			return err
		}
		if done {
			return nil
		}
		time.Sleep(time.Second * 15)
	}
}
