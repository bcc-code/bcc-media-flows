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

func Test_H264(t *testing.T) {
	currentPercent := 0.0

	progressCallback := func(percent float64) {
		currentPercent = percent
		fmt.Println(percent)
	}

	_, err := H264(EncodeInput{
		FilePath:  "/Users/fredrikvedvik/Downloads/test.mkv",
		OutputDir: "/Users/fredrikvedvik/Downloads",
	}, progressCallback)
	assert.Nil(t, err)
	assert.Equal(t, 1.0, currentPercent)
}
