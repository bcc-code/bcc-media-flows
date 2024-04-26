package rclone

import (
	"net/http"
	"strconv"
	"strings"
	"time"
)

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

func CheckJobStatus(jobID int, retries int) (*JobStatus, error) {
	runNr := 1

	req, err := http.NewRequest(http.MethodPost, baseUrl+"/job/status", strings.NewReader(`{"jobid":`+strconv.Itoa(jobID)+`}`))
	if err != nil {
		return nil, err
	}

	var status *JobStatus
	for runNr <= retries {
		status, err = doRequest[JobStatus](req)

		if err == nil {
			break
		}

		runNr++
		time.Sleep(5 * time.Second)
	}

	return status, err
}
