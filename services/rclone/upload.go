package rclone

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/ansel1/merry/v2"
	"github.com/orsinium-labs/enum"
	"net/http"
	"time"
)

// baseUrl is where the rclone remote-control API lives. A var so the tests can point
// the package at a stub.
var baseUrl = "http://rclone.lan.bcc.media"

var (
	errTimeout = merry.Sentinel("timeout waiting for transfer slot")
)

type Priority enum.Member[string]

var (
	PriorityLow    = Priority{Value: "low"}
	PriorityNormal = Priority{Value: "normal"}
	PriorityHigh   = Priority{Value: "high"}

	// Priorities determines the order of priority
	// The leftmost item is the highest priority
	Priorities = enum.New(PriorityHigh, PriorityNormal, PriorityLow)
)

type copyRequest struct {
	Async       bool   `json:"_async"`
	Source      string `json:"srcFs"`
	Destination string `json:"dstFs"`
}

func CopyDir(ctx context.Context, source, destination string) (*JobResponse, error) {
	body, err := json.Marshal(copyRequest{
		Async:       true,
		Source:      source,
		Destination: destination,
	})
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseUrl+"/sync/copy", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	return doRequest[JobResponse](req)
}

type fileRequest struct {
	Async             bool   `json:"_async"`
	SourceRemote      string `json:"srcFs"`
	SourcePath        string `json:"srcRemote"`
	DestinationRemote string `json:"dstFs"`
	DestinationPath   string `json:"dstRemote"`
}

func MoveFile(ctx context.Context, sourceRemote, sourcePath, destinationRemote, destinationPath string, priority Priority) (*JobResponse, error) {
	body, err := json.Marshal(fileRequest{
		Async:             true,
		SourceRemote:      sourceRemote,
		SourcePath:        sourcePath,
		DestinationRemote: destinationRemote,
		DestinationPath:   destinationPath,
	})
	if err != nil {
		return nil, err
	}

	err = waitForTransferSlot(ctx, priority, time.Hour)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseUrl+"/operations/movefile", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	return doRequest[JobResponse](req)
}

func CopyFile(ctx context.Context, sourceRemote, sourcePath, destinationRemote, destinationPath string, priority Priority) (*JobResponse, error) {
	body, err := json.Marshal(fileRequest{
		Async:             true,
		SourceRemote:      sourceRemote,
		SourcePath:        sourcePath,
		DestinationRemote: destinationRemote,
		DestinationPath:   destinationPath,
	})

	if err != nil {
		return nil, err
	}

	err = waitForTransferSlot(ctx, priority, time.Hour)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseUrl+"/operations/copyfile", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	return doRequest[JobResponse](req)
}
