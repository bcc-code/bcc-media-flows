package transcode

import (
	"fmt"
	bccmflows "github.com/bcc-code/bccm-flows"
	"github.com/bcc-code/bccm-flows/common"
	"github.com/bcc-code/bccm-flows/utils"
	"github.com/samber/lo"
	"log"
	"os/exec"
	"path/filepath"
	"strings"
)

type audioFile struct {
	Path     string
	Language string
}

// Order and respect the global language ordering.
func getVideoAudioFiles(input common.MuxInput) []audioFile {
	var languageKeys []string
	for lang := range input.AudioFilePaths {
		languageKeys = append(languageKeys, lang)
	}

	languages := utils.LanguageKeysToOrderedLanguages(languageKeys)

	return lo.Map(languages, func(lang bccmflows.Language, _ int) audioFile {
		return audioFile{
			Path:     input.AudioFilePaths[lang.ISO6391],
			Language: lang.ISO6391,
		}
	})
}

func Mux(input common.MuxInput, progressCallback func(Progress)) (*common.MuxResult, error) {
	//Use ffmpeg to mux the video
	info, err := ProbeFile(input.VideoFilePath)
	if err != nil {
		return nil, err
	}

	outputPath := filepath.Join(input.DestinationPath, input.FileName+".mp4")

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

	params = append(params,
		"-c:v", "copy",
		"-c:a", "copy",
		"-y", outputPath,
	)

	log.Default().Println(strings.Join(params, " "))

	cmd := exec.Command("ffmpeg", params...)

	_, err = utils.ExecuteCmd(cmd, parseProgressCallback(infoToBase(info), progressCallback))

	return nil, err
}
