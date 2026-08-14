package transcode

import (
	"bufio"
	_ "embed"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/bcc-code/bcc-media-flows/paths"
	"github.com/bcc-code/bcc-media-flows/services/ffmpeg"
)

func SubtitleBurnIn(videoFile, subtitleFile, subtitleHeader, outputPath paths.Path, progressCallback ffmpeg.ProgressCallback) (*paths.Path, error) {
	// CreateBurninASSFile returns (nil, err) on four paths, and assFile.Local() below
	// dereferences the result, so this error must be checked.
	assFile, err := CreateBurninASSFile(subtitleHeader, subtitleFile)
	if err != nil {
		return nil, fmt.Errorf("could not create burn-in ASS file for %s: %w", subtitleFile.Local(), err)
	}

	base := videoFile.Base()
	filename := base[0 : len(base)-len(videoFile.Ext())]

	output := outputPath.Append(filename + ".subs" + videoFile.Ext())

	_, err = ffmpeg.Run(ffmpeg.Job{
		Input:  videoFile.Local(),
		Output: output.Local(),
		Args: []string{
			"-vf", "ass=" + assFile.Local(),
			"-c:a", "copy",
		},
	}, progressCallback)
	if err != nil {
		return nil, err
	}

	return &output, nil
}

func CreateBurninASSFile(subtitleHeader, subtitleFile paths.Path) (*paths.Path, error) {
	if subtitleFile.Ext() == ".ass" {
		return &subtitleFile, nil
	}

	out := subtitleFile.Dir().Append(subtitleFile.Base() + ".ass")
	headerData, err := os.ReadFile(subtitleHeader.Local())
	if err != nil {
		return nil, err
	}

	// This intercepts the special case where we need to fix the distance between lines
	if subtitleHeader.Base() == "03-brunstad-to-linjer.ass" {
		return &out, specialASSConverter(string(headerData), subtitleFile.Local(), out.Local(), 0.00011)
	} else if subtitleHeader.Base() == "03-brunstad-led-pc25.ass" {
		return &out, specialASSConverter(string(headerData), subtitleFile.Local(), out.Local(), 0.00005)
	}

	// Converting a subtitle file to ASS, so there is nothing to probe and no
	// progress to report.
	_, err = ffmpeg.Run(ffmpeg.Job{
		Input:  subtitleFile.Local(),
		Output: out.Local(),
		Info:   &ffmpeg.StreamInfo{},
	}, nil)
	if err != nil {
		return nil, err
	}
	subtitleContents, err := os.ReadFile(out.Local())
	if err != nil {
		return nil, err
	}
	eventsTagPassed := false
	var lines []string
	for _, l := range strings.Split(string(subtitleContents), "\n") {
		if strings.HasPrefix(l, "[Events]") {
			eventsTagPassed = true
			continue
		}
		if !eventsTagPassed {
			continue
		}
		lines = append(lines, l)
	}

	err = os.WriteFile(out.Local(), []byte(string(headerData)+"\n"+strings.Join(lines, "\n")), ffmpeg.OutputFileMode)
	if err != nil {
		return nil, err
	}

	return &out, nil
}

// specialASSConverter converts a .srt file to an .ass file, and assures enough spacing between lines
func specialASSConverter(header, inputFile, outputFile string, offset float64) error {
	file, err := os.Open(inputFile)
	if err != nil {
		return err
	}
	defer file.Close()

	outFile, err := os.Create(outputFile)
	if err != nil {
		return err
	}
	defer outFile.Close()

	outFile.WriteString(header)

	scanner := bufio.NewScanner(file)
	var lineCount int
	var startTime, endTime string
	var textLines []string
	timestampPattern := regexp.MustCompile(`(\d{2}:\d{2}:\d{2},\d{3}) --> (\d{2}:\d{2}:\d{2},\d{3})`)

	for scanner.Scan() {
		line := scanner.Text()
		lineCount++
		if lineCount == 1 {
			// Skip the sequence number line
			continue
		}

		if timestampPattern.MatchString(line) {
			matches := timestampPattern.FindStringSubmatch(line)
			startTime = matches[1]
			endTime = matches[2]
			continue
		}

		if line == "" {
			if len(textLines) > 0 {
				writeEvent(outFile, startTime, endTime, textLines, offset)
				textLines = nil
			}
			lineCount = 0
		} else {
			textLines = append(textLines, line)
		}
	}

	// Write the last event if the file doesn't end with a blank line
	if len(textLines) > 0 {
		writeEvent(outFile, startTime, endTime, textLines, offset)
	}

	return err
}

func convertTimestamp(input string) string {
	// Split the time part into hours, minutes, and seconds.milliseconds
	timeParts := strings.Split(input, ":")
	if len(timeParts) != 3 {
		return input // Return the original input if format is incorrect
	}

	secondsParts := strings.Split(timeParts[2], ".")
	if len(secondsParts) != 2 {
		return input // Return the original input if format is incorrect
	}

	// Convert milliseconds to a float and round to 2 decimal places
	secondsFloat, err := strconv.ParseFloat("0."+secondsParts[1], 64)
	if err != nil {
		return input // Return the original input if there's an error parsing milliseconds
	}
	secondsRounded := fmt.Sprintf("%.2f", secondsFloat)[1:] // Trim leading "0"

	// Remove leading zero from hours if present
	hours := strings.TrimPrefix(timeParts[0], "0")
	if hours == "" {
		hours = "0"
	}

	// Combine parts back into the desired format
	return fmt.Sprintf("%s:%s:%s%s", hours, timeParts[1], secondsParts[0], secondsRounded)
}

func writeEvent(outFile *os.File, startTime, endTime string, textLines []string, offset float64) {
	startTime = convertTimeFormat(startTime)
	endTime = convertTimeFormat(endTime)

	var text string
	if len(textLines) == 1 {
		text = textLines[0]
	} else if len(textLines) == 2 {
		text = fmt.Sprintf(`{\org(-2000000,0)\fr%f}%s{\r}\N%s`, offset, textLines[0], textLines[1])
	} else {
		text = strings.Join(textLines, `\N`)
	}

	event := fmt.Sprintf("Dialogue: 0,%s,%s,Default,,0,0,0,,%s\n", startTime, endTime, text)
	outFile.WriteString(event)
}

func convertTimeFormat(srtTime string) string {
	return convertTimestamp(strings.Replace(srtTime[:12], ",", ".", 1))
}

// appendBurnInFilter adds the subtitle burn-in video filter, if there is a
// subtitle to burn in.
func appendBurnInFilter(filters []string, style, subtitle *paths.Path) ([]string, error) {
	if subtitle == nil {
		return filters, nil
	}
	if style == nil {
		return nil, fmt.Errorf("burn-in subtitle %s given with no style", subtitle.Local())
	}

	assFile, err := CreateBurninASSFile(*style, *subtitle)
	if err != nil {
		return nil, err
	}

	return append(filters, "ass="+assFile.Local()), nil
}
