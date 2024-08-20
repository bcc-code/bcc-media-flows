package rclone

import (
	"strconv"
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

func (r *Client) CheckJobStatus(jobID int) (*JobStatus, error) {
	req := r.restyClient.R()
	req.SetBody(`{"jobid":` + strconv.Itoa(jobID) + `}`)
	req.SetResult(&JobStatus{})
	res, err := req.Post("/job/status")

	if err != nil {
		return nil, err
	}

	status := res.Result().(*JobStatus)
	return status, err
}
