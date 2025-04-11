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
	assFile := &subtitleFile
	assFile, err := CreateBurninASSFile(subtitleHeader, subtitleFile)

	params := []string{
		"-i", videoFile.Local(),
		"-vf", "ass=" + assFile.Local(),
		"-c:a", "copy",
	}

	base := videoFile.Base()
	filename := base[0 : len(base)-len(videoFile.Ext())]

	output := outputPath.Append(filename + ".subs" + videoFile.Ext())

	params = append(params, output.Local())

	info, err := ffmpeg.GetStreamInfo(videoFile.Local())
	if err != nil {
		return nil, err
	}

	_, err = ffmpeg.Do(params, info, progressCallback)
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
		return &out, specialASSConverter(string(headerData), subtitleFile.Local(), out.Local())
	}

	_, err = ffmpeg.Do([]string{
		"-y",
		"-i", subtitleFile.Local(),
		out.Local(),
	}, ffmpeg.StreamInfo{}, nil)
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

	err = os.WriteFile(out.Local(), []byte(string(headerData)+"\n"+strings.Join(lines, "\n")), os.ModePerm)
	if err != nil {
		return nil, err
	}

	return &out, nil
}

// specialASSConverter converts a .srt file to an .ass file, and assures enough spacing between lines
func specialASSConverter(header, inputFile, outputFile string) error {
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
				writeEvent(outFile, startTime, endTime, textLines)
				textLines = nil
			}
			lineCount = 0
		} else {
			textLines = append(textLines, line)
		}
	}

	// Write the last event if the file doesn't end with a blank line
	if len(textLines) > 0 {
		writeEvent(outFile, startTime, endTime, textLines)
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

func writeEvent(outFile *os.File, startTime, endTime string, textLines []string) {
	startTime = convertTimeFormat(startTime)
	endTime = convertTimeFormat(endTime)

	var text string
	if len(textLines) == 1 {
		text = textLines[0]
	} else if len(textLines) == 2 {
		text = fmt.Sprintf(`{\org(-2000000,0)\fr0.00011}%s{\r}\N%s`, textLines[0], textLines[1])
	} else {
		text = strings.Join(textLines, `\N`)
	}

	event := fmt.Sprintf("Dialogue: 0,%s,%s,Default,,0,0,0,,%s\n", startTime, endTime, text)
	outFile.WriteString(event)
}

func convertTimeFormat(srtTime string) string {
	return convertTimestamp(strings.Replace(srtTime[:12], ",", ".", 1))
}
