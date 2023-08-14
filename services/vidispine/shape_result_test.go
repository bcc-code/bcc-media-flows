package vidispine

import (
	"testing"
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
	if path != "/path/to/file" {
		t.Error("Path is not correct")
	}
}
