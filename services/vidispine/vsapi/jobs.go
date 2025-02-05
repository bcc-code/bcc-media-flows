package vsapi

import (
	"net/url"
)

type JobsSearchResponse struct {
	Hits int           `json:"hits"`
	Jobs []JobDocument `json:"job"`
}

func (c *Client) FindJob(itemID string, jobType string) (*JobDocument, error) {
	requestURL, _ := url.Parse(c.baseURL)
	requestURL.Path += "/job"

	q := requestURL.Query()
	q.Add("type", jobType)
	q.Add("jobmetadata", "itemId="+itemID)
	q.Add("sort", "startTime asc")

	requestURL.RawQuery = q.Encode()

	result, err := c.restyClient.R().
		SetHeader("Accept", "application/json").
		SetResult(&JobsSearchResponse{}).
		Get(requestURL.String())

	if err != nil {
		return nil, err
	}

	res := result.Result().(*JobsSearchResponse)

	if len(res.Jobs) == 0 {
		return nil, nil
	}

	return &res.Jobs[0], nil
}

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
	JobID    string  `json:"jobId"`
	User     string  `json:"user"`
	Started  *string `json:"started"`
	Finished *string `json:"finished"`
	Status   string  `json:"status"`
	Type     string  `json:"type"`
}
