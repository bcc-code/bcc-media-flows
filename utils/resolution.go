package utils

import "fmt"

var (
	Resolution4K   = MustResolution("3840x2160")
	Resolution1080 = MustResolution("1920x1080")
)

type Resolution struct {
	Width  int
	Height int
	File   bool
}

func ResolutionFromString(str string) (*Resolution, error) {
	var r Resolution
	_, err := fmt.Sscanf(str, "%dx%d", &r.Width, &r.Height)
	if err != nil {
		return nil, fmt.Errorf("failed to parse resolution string %s, err: %v", str, err)
	}
	return &r, nil
}

func MustResolution(str string) *Resolution {
	r, err := ResolutionFromString(str)
	if err != nil {
		panic(err)
	}
	return r
}

func (r *Resolution) FFMpegString() string {
	return fmt.Sprintf("%dx%d", r.Width, r.Height)
}

func (r *Resolution) EnsureEven() {
	if r.Height%2 != 0 {
		r.Height = r.Height + 1
	}

	if r.Width%2 != 0 {
		r.Width = r.Width + 1
	}
}
