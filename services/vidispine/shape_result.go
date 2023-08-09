package vidispine

import (
	"github.com/samber/lo"
)

func (sr ShapeResult) GetShape(tag string) *Shape {
	for _, s := range sr.Shape {
		if lo.Contains(s.Tag, tag) {
			return &s
		}
	}
	return nil
}

func (s Shape) GetPath() string {
	for _, fc := range s.ContainerComponent.File {
		return fc.Path
	}

	return ""
}
