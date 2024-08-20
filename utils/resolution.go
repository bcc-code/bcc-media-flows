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

// ResizeToFit returns the biggest ressolution in the aspect ratio of the source
// that fits into this resolution, while keeping the aspect ratio the same as the source
func (r *Resolution) ResizeToFit(source Resolution) Resolution {
	aspect := float32(source.Width) / float32(source.Height)

	out := Resolution{
		Width:  r.Width,
		Height: r.Height,
		File:   source.File,
	}

	if float32(out.Width)/float32(out.Height) > aspect {
		out.Width = int(float32(out.Height) * aspect)
	} else {
		out.Height = int(float32(out.Width) / aspect)
	}

	return out
}
