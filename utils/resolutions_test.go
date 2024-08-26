package utils

import (
	"fmt"
	"math/rand"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestResolution(t *testing.T) {
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

	for range 10 {
		x := rand.Float32()
		y := rand.Float32()

		resolution, err := ResolutionFromString(fmt.Sprintf("%fx%f", x, y))
		assert.Error(t, err)
		assert.Nil(t, resolution)
	}
}

func TestResolutionToFit(t *testing.T) {
	type testCase struct {
		Source               Resolution
		Target               Resolution
		Expected             Resolution
		ExpectResolutionFlip bool
	}

	testCases := []testCase{

		// Same resolution
		{
			Source: Resolution{
				Width:  1920,
				Height: 1080,
			},
			Target: Resolution{
				Width:  1920,
				Height: 1080,
			},
			Expected: Resolution{
				Width:  1920,
				Height: 1080,
			},
		},

		// Same aspect ratio
		{
			Source: Resolution{
				Width:  1920,
				Height: 1080,
			},
			Target: Resolution{
				Width:  1280,
				Height: 720,
			},
			Expected: Resolution{
				Width:  1280,
				Height: 720,
			},
		},

		// Different aspect ratio
		{
			Source: Resolution{
				Width:  1920,
				Height: 1080,
			},
			Target: Resolution{
				Width:  640,
				Height: 480,
			},
			Expected: Resolution{
				Width:  640,
				Height: 360,
			},
		},

		// Flipped portrait/landscape
		{
			Source: Resolution{
				Width:  1080,
				Height: 1920,
			},
			Target: Resolution{
				Width:  1920,
				Height: 1080,
			},
			Expected: Resolution{
				Width:  1080,
				Height: 1920,
			},
			ExpectResolutionFlip: true,
		},

		// Different aspect ratio + flip
		{
			Source: Resolution{
				Width:  1920,
				Height: 1080,
			},
			Target: Resolution{
				Width:  480,
				Height: 640,
			},
			Expected: Resolution{
				Width:  640,
				Height: 360,
			},
			ExpectResolutionFlip: true,
		},
	}

	for _, tc := range testCases {
		out := tc.Source.ResizedToFit(tc.Target)
		assert.Equal(t, tc.Expected, out)

		if tc.ExpectResolutionFlip {
			tc.Target.Width, tc.Target.Height = tc.Target.Height, tc.Target.Width
		}

		assert.LessOrEqual(t, out.Width, tc.Target.Width)
		assert.LessOrEqual(t, out.Height, tc.Target.Height)

		assert.InDelta(t, float32(out.Width)/float32(out.Height), float32(tc.Source.Width)/float32(tc.Source.Height), 0.01)
	}
}
