package utils

import (
	"bufio"
	"fmt"
	"os/exec"
)

// ExecuteCmd executes the cmd and returns through outputCallback line-by-line before returning the whole stdout at the end.
func ExecuteCmd(cmd *exec.Cmd, outputCallback func(string)) (string, error) {
	stdout, _ := cmd.StdoutPipe()
	stderr, _ := cmd.StderrPipe()

	err := cmd.Start()
	if err != nil {
		return "", fmt.Errorf("start failed %s", err.Error())
	}

	var errorResult string

	go func() {
		scanner := bufio.NewScanner(stderr)
		scanner.Split(bufio.ScanLines)
		for scanner.Scan() {
			line := scanner.Text()
			errorResult += line + "\n"
		}
	}()

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

	err = scanner.Err()
	if err != nil {
		return "", fmt.Errorf("scan failed %s", err.Error())
	}

	err = cmd.Wait()
	if err != nil {
		return "", fmt.Errorf("execution failed error: %s | %s", errorResult, err.Error())
	}

	return result, err
}
