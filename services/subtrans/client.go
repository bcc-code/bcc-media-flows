package subtrans

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/go-resty/resty/v2"

	"github.com/bcc-code/bcc-media-flows/internal/httpx"
)

const serviceName = "subtrans"

type Client struct {
	baseURL     string
	apiKey      string
	restyClient *resty.Client
}

type Config interface {
	BaseURL() string
	APIKey() string
}

func NewClient(cfg Config) *Client {
	baseURL := cfg.BaseURL()
	apiKey := cfg.APIKey()

	client := httpx.New(httpx.Config{
		Service: serviceName,
		BaseURL: baseURL,
		Headers: map[string]string{"accept": "application/json"},
	})

	// Keep the key a query parameter rather than part of a path: that is where
	// httpx.RedactURL looks for it before a URL reaches a workflow history.
	client.SetQueryParam("key", apiKey)

	return &Client{
		baseURL:     baseURL,
		apiKey:      apiKey,
		restyClient: client,
	}
}

// get is the only place this client makes a request, so that no path can return an
// error still carrying the key.
func (c *Client) get(path string, query map[string]string, result any) (*resty.Response, error) {
	req := c.restyClient.R().SetQueryParams(query)
	if result != nil {
		req = req.SetResult(result)
	}

	resp, err := req.Get(path)
	if err != nil {
		return nil, httpx.SanitizeError(err)
	}

	return resp, nil
}

func (c *Client) SearchByName(name string) ([]*SubtransResult, error) {
	res := []*SubtransResult{}
	_, err := c.get("/api/external/story/files/"+name, map[string]string{
		"incLanguages":       "true",
		"returnApprovedOnly": "true",
	}, &res)
	if err != nil {
		// Not the empty slice: callers read that as "this file has no subtitles".
		return nil, err
	}

	return res, nil
}

func (c *Client) SearchByID(id string) (*SubtransResult, error) {
	res := &SubtransResult{}
	_, err := c.get("/api/external/story/storyid/"+id, map[string]string{
		"incLanguages": "true",
	}, res)
	if err != nil {
		return nil, err
	}

	return res, nil
}

func (c *Client) GetFilePrefix(id string) (string, error) {
	res, err := c.SearchByID(id)
	if err != nil {
		return "", err
	}

	return strings.ReplaceAll(res.Name, "%lang%", ""), nil
}

const SubTypeSRT = "srt"
const SubTypeVTT = "vtt"
const SubTypeTxt = "txt"

// BOM is not recommended in UTF-8: https://stackoverflow.com/a/2223926/556085
func stripBOM(fileBytes []byte) []byte {
	trimmedBytes := bytes.Trim(fileBytes, "\ufeff")
	return trimmedBytes
}

func (c *Client) GetSubtitles(id string, format string, approvedOnly bool) (map[string]string, error) {

	subs, err := c.SearchByID(id)
	if err != nil {
		return nil, err
	}

	out := map[string]string{}

	for _, l := range subs.Languages {

		// Norwegian is always approved, even if the system says it's not
		if !l.Approved && l.IsoName != "NOR" {
			continue
		}

		// The 0 is a timecode offset
		url := fmt.Sprintf("/api/external/export/story/storyid/%s/%s/%s/0", id, l.IsoName, format)
		res, err := c.get(url, map[string]string{
			"onlyApproved": fmt.Sprintf("%t", approvedOnly),
		}, nil)
		if err != nil {
			// GetSubtitlesActivity writes every value of the map straight to a .srt,
			// so the body of a failed response must not reach it.
			return nil, err
		}

		out[l.IsoName] = string(stripBOM(res.Body()))
	}

	return out, nil
}

type Language struct {
	Language  string `json:"language"`
	IsoName   string `json:"isoName"`
	SubName   string `json:"subName"`
	Status    int    `json:"status"`
	Completed bool   `json:"completed"`
	Approved  bool   `json:"approved"`
}

type SubtransResult struct {
	Languages []Language `json:"languages"`
	Program   string     `json:"program"`
	Season    int        `json:"season"`
	Episode   int        `json:"episode"`
	Part      int        `json:"part"`
	ID        int        `json:"id"`
	Name      string     `json:"name"`
}
