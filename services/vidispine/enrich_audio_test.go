package vidispine

import (
	"testing"

	"github.com/bcc-code/bcc-media-flows/services/vidispine/vsapi"
	"github.com/bcc-code/bcc-media-flows/services/vidispine/vsmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

// An item with shapes but no "original" among them made GetShape return nil, and
// the AudioComponent access straight after dereferenced it. Every sibling GetShape
// call site in this package guards for nil; this one did not.
func TestEnrichClipWithEmbeddedAudio_NoOriginalShape(t *testing.T) {
	ctrl := gomock.NewController(t)
	vs := vsmock.NewMockClient(ctrl)

	// Shapes exist, just not an "original" one.
	vs.EXPECT().GetShapes("VX-1").Return(&vsapi.ShapeResult{
		Shape: []vsapi.Shape{
			{Tag: []string{"lowres"}},
		},
	}, nil).Times(1)

	clip := &Clip{
		VXID:       "VX-1",
		AudioFiles: map[string]*AudioFile{},
	}

	require.NotPanics(t, func() {
		warnings, err := enrichClipWithEmbeddedAudio(vs, clip, []string{"nor"})

		assert.Error(t, err, "a missing original shape must be reported, not dereferenced")
		assert.Contains(t, err.Error(), "no original shape found")
		assert.Contains(t, err.Error(), "VX-1", "the error should name the item")
		assert.Nil(t, warnings)
	})
}

// No shapes at all takes the same path.
func TestEnrichClipWithEmbeddedAudio_NoShapesAtAll(t *testing.T) {
	ctrl := gomock.NewController(t)
	vs := vsmock.NewMockClient(ctrl)

	vs.EXPECT().GetShapes("VX-2").Return(&vsapi.ShapeResult{}, nil).Times(1)

	clip := &Clip{
		VXID:       "VX-2",
		AudioFiles: map[string]*AudioFile{},
	}

	require.NotPanics(t, func() {
		_, err := enrichClipWithEmbeddedAudio(vs, clip, []string{"nor"})
		assert.Error(t, err)
	})
}
