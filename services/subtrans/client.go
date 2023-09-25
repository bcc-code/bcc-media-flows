package subtrans

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/go-resty/resty/v2"
)

type Client struct {
	baseURL     string
	apiKey      string
	restyClient *resty.Client
}

func NewClient(baseURL string, apiKey string) *Client {
	client := resty.New()
	client.SetBaseURL(baseURL)
	client.SetHeader("accept", "application/json")

	return &Client{
		baseURL:     baseURL,
		apiKey:      apiKey,
		restyClient: client,
	}
}

func (c *Client) SearchByName(name string) ([]*SubtransResult, error) {
	res := []*SubtransResult{}
	_, err := c.restyClient.R().SetResult(&res).Get("/api/external/story/files/" + name + "?incLanguages=true&returnApprovedOnly=true&key=" + c.apiKey)
	return res, err
}

func (c *Client) SearchByID(id string) (*SubtransResult, error) {
	res := &SubtransResult{}
	_, err := c.restyClient.R().SetResult(res).Get("/api/external/story/storyid/" + id + "?incLanguages=true&key=" + c.apiKey)
	return res, err
}

func (c *Client) GetFilePrefix(id string) (string, error) {
	res := &SubtransResult{}
	_, err := c.restyClient.R().SetResult(res).Get("/api/external/story/storyid/" + id + "?incLanguages=true&key=" + c.apiKey)

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
	// The 0 is a timecode offset

	subs, err := c.SearchByID(id)
	if err != nil {
		return nil, err
	}

	out := map[string]string{}

	for _, l := range subs.Languages {
		if !l.Approved {
			continue
		}

		onlyApproved := "onlyApproved="
		if approvedOnly {
			onlyApproved += "true"
		} else {
			onlyApproved += "false"
		}

		url := fmt.Sprintf("/api/external/export/story/storyid/%s/%s/%s/0?key=%s&%s", id, l.IsoName, format, c.apiKey, onlyApproved)
		res, err := c.restyClient.R().Get(url)
		if err != nil {
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
