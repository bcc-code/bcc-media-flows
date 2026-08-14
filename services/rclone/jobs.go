package rclone

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"time"
)

// retryWait is how long CheckJobStatus waits between attempts. A var so the tests can
// drive the loop without waiting five seconds a turn.
var retryWait = 5 * time.Second

type JobStatus struct {
	Duration  float64   `json:"duration"`
	EndTime   time.Time `json:"endTime"`
	Error     string    `json:"error"`
	Finished  bool      `json:"finished"`
	Group     string    `json:"group"`
	ID        int       `json:"id"`
	Output    Output    `json:"output"`
	StartTime time.Time `json:"startTime"`
	Success   bool      `json:"success"`
}

type Output struct {
	Bytes               int64   `json:"bytes"`
	Checks              int     `json:"checks"`
	DeletedDirs         int     `json:"deletedDirs"`
	Deletes             int     `json:"deletes"`
	ElapsedTime         float64 `json:"elapsedTime"`
	Errors              int     `json:"errors"`
	Eta                 int     `json:"eta"`
	FatalError          bool    `json:"fatalError"`
	LastError           string  `json:"lastError"`
	Renames             int     `json:"renames"`
	RetryError          bool    `json:"retryError"`
	ServerSideCopies    int     `json:"serverSideCopies"`
	ServerSideCopyBytes int     `json:"serverSideCopyBytes"`
	ServerSideMoveBytes int     `json:"serverSideMoveBytes"`
	ServerSideMoves     int     `json:"serverSideMoves"`
	Speed               float64 `json:"speed"`
	TotalBytes          int64   `json:"totalBytes"`
	TotalChecks         int     `json:"totalChecks"`
	TotalTransfers      int     `json:"totalTransfers"`
	TransferTime        float64 `json:"transferTime"`
	Transfers           int     `json:"transfers"`
}

type JobResponse struct {
	JobID int `json:"jobid"`
}

type JobStatusRequest struct {
	JobID int `json:"jobid"`
}

func CheckJobStatus(ctx context.Context, jobID int, retries int) (*JobStatus, error) {
	body, err := json.Marshal(JobStatusRequest{JobID: jobID})
	if err != nil {
		return nil, err
	}

	var status *JobStatus
	for attempt := 1; attempt <= retries; attempt++ {
		// The request is rebuilt per attempt: its body is a reader, and a reader that
		// has been sent once is empty, so a retried request asks about no job at all.
		var req *http.Request
		req, err = http.NewRequestWithContext(ctx, http.MethodPost, baseUrl+"/job/status",
			bytes.NewReader(body))
		if err != nil {
			return nil, err
		}

		status, err = doRequest[JobStatus](req)
		if err == nil {
			return status, nil
		}

		if attempt == retries {
			break
		}

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(retryWait):
		}
	}

	return status, err
}
