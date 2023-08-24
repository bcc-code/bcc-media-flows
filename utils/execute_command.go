package utils

import (
	"bufio"
	"fmt"
	"os/exec"
	"time"
)

// ExecuteCmd executes the cmd and returns through outputCallback line-by-line before returning the whole stdout at the end.
func ExecuteCmd(cmd *exec.Cmd, outputCallback func(string)) (string, error) {
	stdout, _ := cmd.StdoutPipe()

	err := cmd.Start()
	if err != nil {
		return "", fmt.Errorf("start failed %s", err.Error())
	}

	var result string
	var currentLine string

	closeChan := make(chan struct{})
	defer close(closeChan)
	timer := time.NewTicker(time.Second * 5)
	go func() {
		for {
			select {
			case <-timer.C:
				fmt.Printf("currentline %s\n", currentLine)
			case <-closeChan:
				return
			}
		}
	}()

	scanner := bufio.NewScanner(stdout)
	scanner.Split(bufio.ScanLines)
	for scanner.Scan() {
		line := scanner.Text()
		result += line + "\n"
		currentLine = fmt.Sprintf("%s, currentline %s\n", time.Now().Format(time.RFC3339), line)
		if outputCallback != nil {
			outputCallback(line)
		}
	}

	err = cmd.Wait()
	fmt.Println("COMMAND EXITED", err)
	if err != nil {
		return "", fmt.Errorf("execution failed error: %s", err.Error())
	}

	return result, err
}
