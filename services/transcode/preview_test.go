//go:build ffmpeg

package transcode

import (
	"testing"
)

func Test_Preview(t *testing.T) {
	_, err := Preview(PreviewInput{
		FilePath:  "",
		OutputDir: "",
	})
	if err != nil {
		t.Error(err)
	}
}
