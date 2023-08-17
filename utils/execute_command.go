package utils

import (
	"bufio"
	"os/exec"
)

// ExecuteCmd executes the cmd and returns through outputCallback line-by-line before returning the whole stdout at the end.
func ExecuteCmd(cmd *exec.Cmd, outputCallback func(string)) (string, error) {
	stdout, _ := cmd.StdoutPipe()

	err := cmd.Start()
	if err != nil {
		return "", err
	}

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
		return "", err
	}

	err = cmd.Wait()
	if err != nil {
		return "", err
	}

	return result, err
}
