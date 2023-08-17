package transcode

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"os"
	"testing"
)

func Test_Preview(t *testing.T) {
	currentPercent := 0.0

	progressCallback := func(percent float64) {
		currentPercent = percent
		fmt.Println(percent)
	}

	_, err := Preview(PreviewInput{
		FilePath:  os.Getenv("TEST_FILEPATH"),
		OutputDir: os.Getenv("TEST_OUTPUTPATH"),
	}, progressCallback)
	assert.Nil(t, err)
	assert.Equal(t, 1.0, currentPercent)
}
