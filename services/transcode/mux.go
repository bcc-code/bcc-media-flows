package transcode

import (
	"fmt"
	bccmflows "github.com/bcc-code/bccm-flows"
	"github.com/bcc-code/bccm-flows/utils"
	"github.com/samber/lo"
	"log"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

type MuxVideoInput struct {
	VideoFilePath   string
	AudioFilePaths  map[string]string
	Width           int
	Height          int
	FrameRate       int
	Bitrate         string
	DestinationPath string
}

type MuxVideoResult struct {
	Path string
}

type audioFile struct {
	Path     string
	Language string
}

// Order and respect the global language ordering.
func getVideoAudioFiles(input MuxVideoInput) []audioFile {
	var languageKeys []string
	for lang := range input.AudioFilePaths {
		languageKeys = append(languageKeys, lang)
	}

	// Do we want this to fail the job if key doesn't exist? Will panic.
	languages := bccmflows.LanguageList(lo.Map(languageKeys, func(key string, _ int) bccmflows.Language {
		return bccmflows.LanguagesByISO[key]
	}))

	// Sort languages by priority
	sort.Sort(languages)

	return lo.Map(languages, func(lang bccmflows.Language, _ int) audioFile {
		return audioFile{
			Path:     input.AudioFilePaths[lang.ISO6391],
			Language: lang.ISO6391,
		}
	})
}

func MuxVideo(input MuxVideoInput, progressCallback func(Progress)) (*MuxVideoResult, error) {
	//Use ffmpeg to mux the video
	info, err := ProbeFile(input.VideoFilePath)
	if err != nil {
		return nil, err
	}

	outputPath := filepath.Join(input.DestinationPath, filepath.Base(input.VideoFilePath))
	// replace extension with .mp4
	outputPath = outputPath[:len(outputPath)-len(filepath.Ext(outputPath))] + ".mp4"

	params := []string{
		"-progress", "pipe:1",
		"-i", input.VideoFilePath,
	}

	audioFiles := getVideoAudioFiles(input)

	for _, af := range audioFiles {
		params = append(params,
			"-i", af.Path,
		)
	}

	params = append(params, "-map", "0:v")

	for index, af := range audioFiles {
		params = append(params,
			"-map", fmt.Sprintf("%d:a", index+1),
			fmt.Sprintf("-metadata:s:%d", index+1), fmt.Sprintf("language=%s", af.Language),
		)
	}

	// Letterbox and padding
	if input.Width != 0 && input.Height != 0 {
		params = append(params,
			"-vf",
			fmt.Sprintf("scale=%[1]d:%[2]d:force_original_aspect_ratio=decrease,pad=%[1]d:%[2]d:(ow-iw)/2:(oh-ih)/2",
				input.Width,
				input.Height,
			),
		)
	}

	params = append(params,
		"-c:v", "h264_videotoolbox",
		"-b:v", input.Bitrate,
		"-r", fmt.Sprintf("%d", input.FrameRate),
		"-c:a", "aac",
		"-b:a", "256k",
		"-y", outputPath)

	log.Default().Println(strings.Join(params, " "))

	cmd := exec.Command("ffmpeg", params...)

	_, err = utils.ExecuteCmd(cmd, parseProgressCallback(info, progressCallback))

	return nil, err
}
