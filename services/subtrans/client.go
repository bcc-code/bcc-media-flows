package subtrans

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/go-resty/resty/v2"

	"github.com/bcc-code/bcc-media-flows/services/internal/httpx"
)

// serviceName names Subtrans in the errors this client returns.
const serviceName = "subtrans"

type Client struct {
	baseURL     string
	apiKey      string
	restyClient *resty.Client
}

func NewClient(baseURL string, apiKey string) *Client {
	client := httpx.New(httpx.Config{
		Service: serviceName,
		BaseURL: baseURL,
		Headers: map[string]string{"accept": "application/json"},
	})

	// The key authenticates every request. Setting it on the client rather than
	// pasting it into each path keeps it in the query parameters, where the error
	// redaction can find it before the URL reaches a workflow history.
	client.SetQueryParam("key", apiKey)

	return &Client{
		baseURL:     baseURL,
		apiKey:      apiKey,
		restyClient: client,
	}
}

func (c *Client) SearchByName(name string) ([]*SubtransResult, error) {
	res := []*SubtransResult{}
	_, err := c.restyClient.R().
		SetQueryParams(map[string]string{
			"incLanguages":       "true",
			"returnApprovedOnly": "true",
		}).
		SetResult(&res).
		Get("/api/external/story/files/" + name)
	if err != nil {
		// Not the empty slice: the caller reads that as "no subtitles exist for this
		// file", which for GetOrCreateSubtransID means either failing the ingest as
		// non-retryable or continuing without subtitles.
		return nil, err
	}

	return res, nil
}

func (c *Client) SearchByID(id string) (*SubtransResult, error) {
	res := &SubtransResult{}
	_, err := c.restyClient.R().
		SetQueryParam("incLanguages", "true").
		SetResult(res).
		Get("/api/external/story/storyid/" + id)
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
		res, err := c.restyClient.R().
			SetQueryParam("onlyApproved", fmt.Sprintf("%t", approvedOnly)).
			Get(url)
		if err != nil {
			// The body of a failed response is what would otherwise be written to
			// disk as the subtitle file: GetSubtitlesActivity writes every value of
			// this map straight to a .srt.
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
