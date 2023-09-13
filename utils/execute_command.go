package utils

import (
	"bufio"
	"fmt"
	"log"
	"os/exec"
)

// ExecuteCmd executes the cmd and returns through outputCallback line-by-line before returning the whole stdout at the end.
func ExecuteCmd(cmd *exec.Cmd, outputCallback func(string)) (string, error) {
	stdout, _ := cmd.StdoutPipe()
	//stderr, _ := cmd.StderrPipe()

	log.Default()

	err := cmd.Start()
	if err != nil {
		return "", fmt.Errorf("start failed %s", err.Error())
	}

	var errorString string

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
		return "", fmt.Errorf("execution failed error: %s,\nmessage: %s", err.Error(), errorString)
	}

	return result, err
}
