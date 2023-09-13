package vidispine

import (
	"github.com/samber/lo"
	"net/url"
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
	// Cut off the "file://" prefix
	for _, fc := range s.ContainerComponent.File {
		p, _ := url.PathUnescape(fc.URI[0][7:])
		return p
	}

	// Does this make sense, can it be multiple files???
	for _, bc := range s.BinaryComponent {
		for _, f := range bc.File {
			p, _ := url.PathUnescape(f.URI[0][7:])
			return p
		}
	}

	return ""
}
