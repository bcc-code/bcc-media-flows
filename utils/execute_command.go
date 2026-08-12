package utils

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os/exec"
	"strings"
)

// maxScanLine is the largest single output line we accept. bufio.Scanner defaults
// to 64 KiB, which ffmpeg can exceed when it dumps metadata-heavy stream info.
const maxScanLine = 1024 * 1024

// maxErrorTail is how much of a failed command's stderr we quote back. Errors from
// activities end up in the Temporal workflow history, so this must stay bounded —
// a decode-warning storm can run to megabytes.
const maxErrorTail = 4096

// newLineScanner returns a line scanner with a raised line limit.
func newLineScanner(r io.Reader) *bufio.Scanner {
	scanner := bufio.NewScanner(r)
	scanner.Split(bufio.ScanLines)
	scanner.Buffer(make([]byte, 0, 64*1024), maxScanLine)
	return scanner
}

// tailForError trims s to its last maxErrorTail bytes, keeping the end because that
// is where a failing command reports why.
func tailForError(s string) string {
	if len(s) <= maxErrorTail {
		return s
	}
	return "...(truncated)...\n" + s[len(s)-maxErrorTail:]
}

// ExecuteCmd executes the cmd and returns through outputCallback line-by-line before returning the whole stdout at the end.
func ExecuteCmd(cmd *exec.Cmd, outputCallback func(string)) (string, error) {
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return "", fmt.Errorf("could not open stdout pipe: %w", err)
	}

	errorBytes := bytes.Buffer{}
	cmd.Stderr = &errorBytes

	println("FFMPEG Command:", cmd.String())

	err = cmd.Start()
	if err != nil {
		return "", fmt.Errorf("start failed %s", err.Error())
	}

	var result string

	scanner := newLineScanner(stdout)
	for scanner.Scan() {
		line := scanner.Text()
		result += line + "\n"
		if outputCallback != nil {
			outputCallback(line)
		}
	}
	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("reading stdout failed: %w", err)
	}

	err = cmd.Wait()
	if err != nil {
		return "", fmt.Errorf("execution failed error: %s,\nmessage: %s", err.Error(), tailForError(errorBytes.String()))
	}

	return result, err
}

// ExecuteAnalysisCmd executes the cmd and returns the JSON object the command
// printed to stderr, which is how ffmpeg's loudnorm filter reports its analysis.
func ExecuteAnalysisCmd(cmd *exec.Cmd, outputCallback func(string)) (string, error) {
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return "", fmt.Errorf("could not open stdout pipe: %w", err)
	}

	// Buffered rather than piped, which is what keeps this from deadlocking.
	//
	// Stdout is drained to EOF below, and stdout only reaches EOF once the child
	// exits. Stderr therefore cannot be a pipe read after that drain: a chatty child
	// would fill the 64 KiB stderr pipe, block in write(), never exit, never close
	// stdout, and the drain would never finish either. That is reachable in practice:
	// ffmpeg stays quiet on a clean file thanks to -hide_banner and -nostats, but
	// damaged input produces per-frame decode warnings that run to megabytes.
	//
	// Assigning an io.Writer instead makes os/exec copy stderr on its own goroutine,
	// so it can never fill up. cmd.Wait is what guarantees that copy has finished,
	// so errorBytes must not be read until after it returns.
	errorBytes := bytes.Buffer{}
	cmd.Stderr = &errorBytes

	err = cmd.Start()
	if err != nil {
		return "", fmt.Errorf("start failed %s", err.Error())
	}

	scannerOut := newLineScanner(stdout)
	for scannerOut.Scan() {
		line := scannerOut.Text()
		if outputCallback != nil {
			outputCallback(line)
		}
	}
	if err := scannerOut.Err(); err != nil {
		return "", fmt.Errorf("reading stdout failed: %w", err)
	}

	err = cmd.Wait()
	if err != nil {
		return "", fmt.Errorf("execution failed error: %s,\nmessage: %s", err.Error(), tailForError(errorBytes.String()))
	}

	result, err := extractJSONObject(&errorBytes)
	if err != nil {
		return "", fmt.Errorf("reading stderr failed: %w", err)
	}

	// replace -Inf with -99 if the audio was silent
	result = strings.ReplaceAll(result, "\"-inf\"", "\"-99\"")

	// replace inf with 0 target_offset if the audio was silent
	result = strings.ReplaceAll(result, "\"inf\"", "\"0\"")

	return result, nil
}

// extractJSONObject returns the lines between a bare "{" and a bare "}", which is
// how ffmpeg brackets the loudnorm JSON summary among its other stderr output.
func extractJSONObject(r io.Reader) (string, error) {
	var out strings.Builder
	jsonActive := false

	scanner := newLineScanner(r)
	for scanner.Scan() {
		line := scanner.Text()

		if line == "{" {
			jsonActive = true
		}

		if jsonActive {
			out.WriteString(line)
			out.WriteString("\n")
		}

		if line == "}" {
			jsonActive = false
		}
	}
	if err := scanner.Err(); err != nil {
		return "", err
	}

	return out.String(), nil
}
