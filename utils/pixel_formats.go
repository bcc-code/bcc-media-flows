package utils

import "github.com/samber/lo"

// Extracted from ffprobe -v 0 -show_entries pixel_format=name:flags=alpha -of compact=p=0
var alphaPixelFormats = []string{
	"pal8",
	"argb",
	"rgba",
	"abgr",
	"bgra",
	"yuva420p",
	"ya8",
	"yuva422p",
	"yuva444p",
	"yuva420p9be",
	"yuva420p9le",
	"yuva422p9be",
	"yuva422p9le",
	"yuva444p9be",
	"yuva444p9le",
	"yuva420p10be",
	"yuva420p10le",
	"yuva422p10be",
	"yuva422p10le",
	"yuva444p10be",
	"yuva444p10le",
	"yuva420p16be",
	"yuva420p16le",
	"yuva422p16be",
	"yuva422p16le",
	"yuva444p16be",
	"yuva444p16le",
	"rgba64be",
	"rgba64le",
	"bgra64be",
	"bgra64le",
	"ya16be",
	"ya16le",
	"gbrap",
	"gbrap16be",
	"gbrap16le",
	"ayuv64le",
	"ayuv64be",
	"gbrap12be",
	"gbrap12le",
	"gbrap10be",
	"gbrap10le",
	"gbrapf32be",
	"gbrapf32le",
	"yuva422p12be",
	"yuva422p12le",
	"yuva444p12be",
	"yuva444p12le",
	"vuya",
	"rgbaf16be",
	"rgbaf16le",
	"rgbaf32be",
	"rgbaf32le",
}

func IsAlphaPixelFormat(format string) bool {
	return lo.Contains(alphaPixelFormats, format)
}
