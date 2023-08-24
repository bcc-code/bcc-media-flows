package utils

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"os/exec"
	"time"
)

// ExecuteCmd executes the cmd and returns through outputCallback line-by-line before returning the whole stdout at the end.
func ExecuteCmd(cmd *exec.Cmd, outputCallback func(string)) (string, error) {
	stdout, _ := cmd.StdoutPipe()
	stderr, _ := cmd.StderrPipe()

	err := cmd.Start()
	if err != nil {
		return "", fmt.Errorf("start failed %s", err.Error())
	}

	file, err := os.OpenFile("/tmp/ffmpeg.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return "", fmt.Errorf("couldn't open file %s", err.Error())
	}
	defer file.Close()
	logger := log.New(file, "prefix", log.LstdFlags)

	var errorResult string

	go func() {
		scanner := bufio.NewScanner(stderr)
		scanner.Split(bufio.ScanLines)
		for scanner.Scan() {
			fmt.Println("PRINTED ERR LINE")
			line := scanner.Text()
			fmt.Println(line)
			errorResult += line + "\n"
			_, _ = file.WriteString(line + "\n")
			logger.Println(line)
		}

		err = scanner.Err()
		if err != nil {
			errorResult += fmt.Sprintf("\nscan failed %s", err.Error())
		}
	}()

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

	go func() {
		scanner := bufio.NewScanner(stdout)
		scanner.Split(bufio.ScanLines)
		for scanner.Scan() {
			line := scanner.Text()
			result += line + "\n"
			currentLine = fmt.Sprintf("%s, currentline %s\n", time.Now().Format(time.RFC3339), line)
			if outputCallback != nil {
				outputCallback(line)
				logger.Println(line)
			}
		}

		err = scanner.Err()
		if err != nil {
			errorResult += fmt.Sprintf("\nscan failed %s", err.Error())
		}
	}()

	err = cmd.Wait()
	fmt.Println("COMMAND EXITED", err)
	if err != nil {
		return "", fmt.Errorf("execution failed error: %s | %s", errorResult, err.Error())
	}

	return result, err
}
