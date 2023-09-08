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

type languageFile struct {
	Path     string
	Language string
}

// Order and respect the global language ordering.
func getMuxAudioFiles(input common.MuxInput) []languageFile {
	languages := utils.LanguageKeysToOrderedLanguages(lo.Keys(input.AudioFilePaths))

	return lo.Map(languages, func(lang bccmflows.Language, _ int) languageFile {
		return languageFile{
			Path:     input.AudioFilePaths[lang.ISO6391],
			Language: lang.ISO6391,
		}
	})
}

// Order and respect the global language ordering.
func getMuxSubtitleFiles(input common.MuxInput) []languageFile {
	languages := utils.LanguageKeysToOrderedLanguages(lo.Keys(input.SubtitleFilePaths))

	return lo.Map(languages, func(lang bccmflows.Language, _ int) languageFile {
		return languageFile{
			Path:     input.SubtitleFilePaths[lang.ISO6391],
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

	audioFiles := getMuxAudioFiles(input)
	subtitleFiles := getMuxSubtitleFiles(input)

	for _, f := range audioFiles {
		params = append(params,
			"-i", f.Path,
		)
	}

	for _, f := range subtitleFiles {
		params = append(params,
			"-i", f.Path,
		)
	}

	streams := 0
	params = append(params, "-map", fmt.Sprintf("%d:v", streams))
	streams++

	for _, f := range audioFiles {
		params = append(params,
			"-map", fmt.Sprintf("%d:a", streams),
			fmt.Sprintf("-metadata:s:%d", streams), fmt.Sprintf("language=%s", f.Language),
		)
		streams++
	}

	for _, f := range subtitleFiles {
		params = append(params,
			"-map", fmt.Sprintf("%d:s", streams),
			fmt.Sprintf("-metadata:s:%d", streams), fmt.Sprintf("language=%s", f.Language),
		)
		streams++
	}

	params = append(params,
		"-c:v", "copy",
		"-c:a", "copy",
		"-c:s", "mov_text",
		"-y", outputPath,
	)

	cmd := exec.Command("ffmpeg", params...)

	_, err = utils.ExecuteCmd(cmd, parseProgressCallback(infoToBase(info), progressCallback))

	if err != nil {
		log.Default().Println("mux failed", err)
		return nil, fmt.Errorf("mux failed, %s", strings.Join(params, " "))
	}
	return &common.MuxResult{
		Path: outputPath,
	}, nil
}
