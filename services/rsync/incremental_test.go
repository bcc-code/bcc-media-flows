package rsync

import (
	"testing"

	"github.com/bcc-code/bcc-media-flows/paths"
	"github.com/stretchr/testify/assert"
)

func Test_Incremental(t *testing.T) {
	from := paths.MustParse("/mnt/temp/in.mp4")
	to := paths.MustParse("/mnt/temp/out.mp4")

	err := IncrementalCopy(from, to, func(FileInfo) {})
	assert.Nil(t, err)
}
