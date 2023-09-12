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
	Id        int       `json:"id"`
	StartTime time.Time `json:"startTime"`
	Success   bool      `json:"success"`
}

type JobResponse struct {
	JobID int `json:"jobid"`
}

func CheckJobStatus(jobID int) (*JobStatus, error) {
	req, err := http.NewRequest(http.MethodPost, baseUrl+"/job/status", strings.NewReader(`{"jobid":`+strconv.Itoa(jobID)+`}`))
	if err != nil {
		return nil, err
	}

	return doRequest[JobStatus](req)
}
