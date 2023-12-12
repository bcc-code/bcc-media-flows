package utils

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"

	"github.com/samber/lo"
)

var alphanumericalRegex = regexp.MustCompile("^[a-zA-Z0-9_]+$")

var imageExtensions = []string{
	".png",
	".jpg",
	".jpeg",
}

var presentationExtensions = []string{
	".pptx",
	".key",
}

var mediaExtensions = []string{
	".mxf",
	".mov",
	".wav",
}

var supportedExtensions = append(imageExtensions, append(presentationExtensions, mediaExtensions...)...)

func ValidRawFilename(filename string) bool {
	extension := filepath.Ext(filename)
	base := filepath.Base(filename)
	name := base[:len(base)-len(extension)]
	return alphanumericalRegex.MatchString(name) && lo.Contains(supportedExtensions, extension)
}

func IsMedia(filename string) bool {
	extension := filepath.Ext(filename)
	return lo.Contains(mediaExtensions, extension)
}

func JsonFileToStruct(jsonFile string, output interface{}) error {
	bytes, err := os.ReadFile(jsonFile)
	if err != nil {
		return err
	}
	return json.Unmarshal(bytes, output)
}
