package rclone

import (
	"bytes"
	"encoding/json"
	"net/http"
)

const baseUrl = "http://rclone.lan.bcc.media"

type copyRequest struct {
	Async       bool   `json:"_async"`
	Source      string `json:"srcFs"`
	Destination string `json:"dstFs"`
}

func CopyDir(source, destination string) (*JobResponse, error) {
	body, err := json.Marshal(copyRequest{
		Async:       true,
		Source:      source,
		Destination: destination,
	})
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", baseUrl+"/sync/copy", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	return doRequest[JobResponse](req)
}

type moveRequest struct {
	Async             bool   `json:"_async"`
	SourceRemote      string `json:"srcFs"`
	SourcePath        string `json:"srcRemote"`
	DestinationRemote string `json:"dstFs"`
	DestinationPath   string `json:"dstRemote"`
}

func MoveFile(sourceRemote, sourcePath, destinationRemote, destinationPath string) (*JobResponse, error) {
	body, err := json.Marshal(moveRequest{
		Async:             true,
		SourceRemote:      sourceRemote,
		SourcePath:        sourcePath,
		DestinationRemote: destinationRemote,
		DestinationPath:   destinationPath,
	})
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", baseUrl+"/operations/movefile", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	return doRequest[JobResponse](req)
}
