package utils

import (
	"fmt"
)

var (
	Resolution4K   = MustResolution("3840x2160")
	Resolution1080 = MustResolution("1920x1080")
)

type Resolution struct {
	Width  int
	Height int
	IsFile bool
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

// ResizeToFit returns the biggest resolution in the aspect ratio of the source
// that fits into this resolution, while keeping the aspect ratio the same as the source
func (r *Resolution) ResizedToFit(target Resolution) Resolution {
	tAspect := float32(target.Width) / float32(target.Height)
	sAspect := float32(r.Width) / float32(r.Height)

	// If the target and source are in diferent modes (landscape vs portrait)
	// then rotate the target in order to fit the source better into the target
	//
	// The main use case here is shorts
	flip := tAspect < 1 && sAspect > 1 || tAspect > 1 && sAspect < 1
	tempRes := *r

	if flip {
		tempRes.Width, tempRes.Height = tempRes.Height, tempRes.Width
		sAspect = float32(tempRes.Width) / float32(tempRes.Height)
	}

	out := Resolution{
		Width:  target.Width,
		Height: target.Height,
		IsFile: r.IsFile,
	}

	if tAspect > sAspect {
		out.Width = int(float32(target.Height) * sAspect)
	} else {
		out.Height = int(float32(target.Width) / sAspect)
	}

	if flip {
		out.Width, out.Height = out.Height, out.Width
	}

	return out
}
