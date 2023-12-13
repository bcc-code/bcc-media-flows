package transcode

import (
	"testing"

	"github.com/bcc-code/bccm-flows/paths"
)

func Test_MultitrackMux(t *testing.T) {
	files := []paths.Path{
		paths.MustParse("/mnt/temp/test1.wav"),
		paths.MustParse("/mnt/temp/test2.wav"),
		paths.MustParse("/mnt/temp/test3.wav"),
	}

	_, _ = MultitrackMux(files, files[0].Dir().Append("test_out2.mp4"), nil)
}
