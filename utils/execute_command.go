package utils

import (
	"bufio"
	"bytes"
	"fmt"
	"os/exec"

	"github.com/davecgh/go-spew/spew"
)

// ExecuteCmd executes the cmd and returns through outputCallback line-by-line before returning the whole stdout at the end.
func ExecuteCmd(cmd *exec.Cmd, outputCallback func(string)) (string, error) {
	stdout, _ := cmd.StdoutPipe()
	//stderr, _ := cmd.StderrPipe()

	errorBytes := bytes.Buffer{}
	cmd.Stderr = &errorBytes

	err := cmd.Start()
	if err != nil {
		return "", fmt.Errorf("start failed %s", err.Error())
	}

	//go func() {
	//	scanner := bufio.NewScanner(stderr)
	//	scanner.Split(bufio.ScanLines)
	//	for scanner.Scan() {
	//		errorString += scanner.Text() + "\n"
	//	}
	//}()

	var result string

	scanner := bufio.NewScanner(stdout)
	scanner.Split(bufio.ScanLines)
	for scanner.Scan() {
		line := scanner.Text()
		result += line + "\n"
		if outputCallback != nil {
			outputCallback(line)
		}
	}

	err = cmd.Wait()
	if err != nil {
		return "", fmt.Errorf("execution failed error: %s,\nmessage: %s", err.Error(), errorBytes.String())
	}

	return result, err
}

// ExecuteAnalysisCmd executes the cmd and returns through outputCallback line-by-line and retrutning only
// the part that is a valid JSON string
func ExecuteAnalysisCmd(cmd *exec.Cmd, outputCallback func(string)) (string, error) {
	stderr, _ := cmd.StderrPipe()

	err := cmd.Start()
	if err != nil {
		return "", fmt.Errorf("start failed %s", err.Error())
	}

	var result string

	jsonStarted := false

	scanner := bufio.NewScanner(stderr)
	scanner.Split(bufio.ScanLines)
	for scanner.Scan() {
		spew.Dump(scanner.Text())
		line := scanner.Text()

		if line == "{" {
			jsonStarted = true
		}

		if jsonStarted {
			result += line + "\n"
		}

		if outputCallback != nil {
			outputCallback(line)
		}
	}

	err = cmd.Wait()
	if err != nil {
		return "", fmt.Errorf("execution failed error: %s", err.Error())
	}

	return result, err
}
