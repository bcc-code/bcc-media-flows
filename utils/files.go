package utils

import (
	"github.com/samber/lo"
	"path/filepath"
	"regexp"
)

var alphanumericalRegex = regexp.MustCompile("^[a-zA-Z0-9_]+$")

var supportedExtensions = []string{
	".mxf",
	".mov",
	".png",
	".jpg",
	".jpeg",
	".wav",
}

func ValidRawFilename(filename string) bool {
	extension := filepath.Ext(filename)
	name := filename[:len(filename)-len(extension)]
	return alphanumericalRegex.MatchString(name) && lo.Contains(supportedExtensions, extension)
}
