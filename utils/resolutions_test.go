package utils

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"math/rand"
	"testing"
)

func TestResolutionFromString(t *testing.T) {
	for range 10 {
		x := rand.Int()
		y := rand.Int()

		resolution, err := ResolutionFromString(fmt.Sprintf("%dx%d", x, y))
		assert.NoError(t, err)
		assert.Equal(t, x, resolution.Width)
		assert.Equal(t, y, resolution.Height)

		assert.Equal(t, resolution.FFMpegString(), fmt.Sprintf("%dx%d", x, y))

		MustResolution(fmt.Sprintf("%dx%d", x, y))

		resolution.EnsureEven()

		assert.Equal(t, resolution.Height%2, 0)
		assert.Equal(t, resolution.Width%2, 0)
	}
}
