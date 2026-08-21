package testutils

import (
	"os/exec"
	"strings"
	"sync"
	"testing"
)

var ffmpegEncoders = sync.OnceValue(func() string {
	out, err := exec.Command("ffmpeg", "-hide_banner", "-encoders").Output()
	if err != nil {
		return ""
	}
	return string(out)
})

// SkipWithoutEncoder skips the test when the local ffmpeg build does not
// include the named encoder, e.g. the non-free libfdk_aac.
func SkipWithoutEncoder(t testing.TB, encoder string) {
	if !strings.Contains(ffmpegEncoders(), " "+encoder+" ") {
		t.Skipf("ffmpeg encoder %s not available", encoder)
	}
}
