package utils

import (
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

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
	extension = strings.ToLower(extension)
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

func GetOldFile(rootDir string, olderThan time.Time) ([]string, error) {
	//olderThan := time.Now().AddDate(0, 0, -14)
	older := []string{}

	err := filepath.WalkDir(rootDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if !d.IsDir() {
			info, err := d.Info()
			if err != nil {
				return err
			}
			if info.ModTime().Before(olderThan) {
				older = append(older, path)
			}
		}
		return nil
	})
	return older, err
}

func GetEmptyDirs(rootDir string) ([]string, error) {
	empty := []string{}
	err := filepath.WalkDir(rootDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() {
			isEmpty, err := IsDirEmpty(path)
			if err != nil {
				return err
			}

			if isEmpty {
				empty = append(empty, path)
			}
		}

		return nil
	})
	return empty, err
}

func IsDirEmpty(dir string) (bool, error) {
	f, err := os.Open(dir)
	if err != nil {
		return false, err
	}
	defer f.Close()

	names, err := f.Readdirnames(1) // Try to read at least one entry
	if err != nil && len(names) == 0 {
		return true, nil
	}

	return false, err
}
