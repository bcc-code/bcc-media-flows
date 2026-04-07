package vsapi

import (
	"errors"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_parseVSError_ShapeTagNotFound(t *testing.T) {
	body := []byte(`{"notFound":{"type":"shape-tag","id":"mul_yue_low","context":null},"internalServer":null,"forbidden":null,"notYetImplemented":null,"conflict":null,"invalidInput":null,"licenseFault":null,"fileAlreadyExists":null,"notAuthorized":null}`)

	err := parseVSError(body, http.StatusNotFound, "mul_yue_low", "VX-123")

	assert.Error(t, err)
	assert.True(t, errors.Is(err, ErrShapeTagNotFound), "expected error to wrap ErrShapeTagNotFound")
	assert.Contains(t, err.Error(), "mul_yue_low")
	assert.Contains(t, err.Error(), "VX-123")
}

func Test_parseVSError_OtherEnvelope(t *testing.T) {
	body := []byte(`{"notFound":null,"forbidden":{},"internalServer":null}`)

	err := parseVSError(body, http.StatusForbidden, "lowres_watermarked", "VX-456")

	assert.Error(t, err)
	assert.False(t, errors.Is(err, ErrShapeTagNotFound), "non-shape-tag errors must not wrap ErrShapeTagNotFound")
	assert.Contains(t, err.Error(), "403")
	assert.Contains(t, err.Error(), "lowres_watermarked")
	assert.Contains(t, err.Error(), "VX-456")
}

func Test_parseVSError_NonJSONBody(t *testing.T) {
	body := []byte(`<html>internal server error</html>`)

	err := parseVSError(body, http.StatusInternalServerError, "tag", "VX-789")

	assert.Error(t, err)
	assert.False(t, errors.Is(err, ErrShapeTagNotFound))
	assert.Contains(t, err.Error(), "500")
	assert.Contains(t, err.Error(), "<html>internal server error</html>")
}

func Test_GetPath(t *testing.T) {
	sr := ShapeResult{
		Shape: []Shape{
			{
				Tag: []string{"tag1", "tag2"},
				ContainerComponent: ContainerComponent{
					File: []File{
						{
							URI: []string{"file:///path/to/file"},
						},
					},
				},
			},
		},
	}

	path := sr.GetShape("tag1").GetPath()
	assert.Equal(t, "/path/to/file", path)
}
