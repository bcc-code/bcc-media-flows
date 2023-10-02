package vsapi

import (
	"net/url"
	"time"
)

func (c *Client) GetJob(jobID string) (*JobDocument, error) {
	requestURL, _ := url.Parse(c.baseURL)
	requestURL.Path += "/job/" + url.PathEscape(jobID)
	q := requestURL.Query()
	requestURL.RawQuery = q.Encode()

	result, err := c.restyClient.R().
		SetHeader("Accept", "application/json").
		SetResult(&JobDocument{}).
		Get(requestURL.String())

	if err != nil {
		return nil, err
	}
	return result.Result().(*JobDocument), nil
}

type JobDocument struct {
	JobID    string     `json:"jobId"`
	User     string     `json:"user"`
	Started  *time.Time `json:"started"`
	Finished *time.Time `json:"finished"`
	Status   string     `json:"status"`
	Type     string     `json:"type"`
}
