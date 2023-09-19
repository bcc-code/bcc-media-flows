package vsapi

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

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
