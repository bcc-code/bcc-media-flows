package vsapi

import (
	"bytes"
	"errors"
	"fmt"
	"net/url"
	"strconv"

	"github.com/bcc-code/bcc-media-flows/services/vidispine/vscommon"
)

type MetadataField struct {
	End   string `json:"end"`
	Start string `json:"start"`
	UUID  string `json:"uuid"`
	Value string `json:"value"`
}

type MetadataResult struct {
	Terse map[string]([]*MetadataField) `json:"terse"`
	ID    string                        `json:"id"`
}

// Get returns the first value of the given key, or the fallback if the key is not present
// It does not check what clip the metadata belongs to!
func (m *MetadataResult) Get(key vscommon.FieldType, fallback string) string {
	if val, ok := m.Terse[key.Value]; !ok {
		return fallback
	} else if len(val) == 0 {
		return fallback
	} else {
		return val[0].Value
	}
}

func (m *MetadataResult) GetArray(key vscommon.FieldType) []string {
	if val, ok := m.Terse[key.Value]; !ok {
		return []string{}
	} else {
		out := []string{}
		for _, v := range val {
			out = append(out, v.Value)
		}
		return out
	}
}

func (c *Client) GetMetadata(vsID string) (*MetadataResult, error) {
	url := c.baseURL + "/item/" + vsID + "?content=metadata&terse=true"

	resp, err := c.restyClient.R().
		SetResult(&MetadataResult{}).
		Get(url)

	if err != nil {
		return nil, err
	}

	return resp.Result().(*MetadataResult), nil
}

func (c *Client) setItemMetadataField(itemID, startTC, endTC, group, key, value string, add bool) error {
	requestURL, _ := url.Parse(c.baseURL)
	requestURL.Path += fmt.Sprintf("/item/%s/metadata", url.PathEscape(itemID))
	q := requestURL.Query()
	requestURL.RawQuery = q.Encode()

	if startTC == "" {
		startTC = "-INF"
	}
	if endTC == "" {
		endTC = "+INF"
	}

	var body bytes.Buffer
	err := xmlSetMetadataPlaceholderTmpl.Execute(&body, struct {
		StartTC string
		EndTC   string
		Group   string
		Key     string
		Value   string
		Add     bool
	}{
		startTC,
		endTC,
		group,
		key,
		value,
		add,
	})
	if err != nil {
		return err
	}

	_, err = c.restyClient.R().
		SetHeader("content-type", "application/xml").
		SetBody(body.String()).
		Put(requestURL.String())

	if err != nil {
		return err
	}

	return nil
}
func (c *Client) SetItemMetadataField(itemID, group, key, value string) error {
	return c.setItemMetadataField(itemID, MinusInf, PlusInf, group, key, value, false)
}

func (c *Client) AddToItemMetadataField(itemID, group, key, value string) error {
	return c.setItemMetadataField(itemID, MinusInf, PlusInf, group, key, value, true)
}

func (c *Client) SetItemMetadataFieldWithTC(itemID, startTC, endTC, group, key, value string) error {
	return c.setItemMetadataField(itemID, startTC, endTC, group, key, value, false)
}

func (c *Client) AddToItemMetadataFieldWithTC(itemID, startTC, endTC, group, key, value string) error {
	return c.setItemMetadataField(itemID, startTC, endTC, group, key, value, true)
}

// GetInOut returns the in and out point of the clip in seconds, suitable
// for use with ffmpeg
func (m *MetadataResult) GetInOut(beginTC string) (float64, float64, error) {
	var v *MetadataField
	if val, ok := m.Terse[vscommon.FieldTitle.Value]; !ok {
		// This should not happen as everything should have a title
		return 0, 0, errors.New("Missing title")
	} else {
		v = val[0]
	}

	start := 0.0
	if v.Start == MinusInf && v.End == PlusInf {
		// This is a full asset so we return 0.0 start and the lenght of the asset as end
		endString := m.Get(vscommon.FieldDurationSeconds, "0")
		end, err := strconv.ParseFloat(endString, 64)
		return start, end, err
	}

	// Now we are in subclip territory. Here we need to extract the TC of the in and out point
	// and convert it to seconds for use with ffmpeg

	inTCseconds, err := vscommon.TCToSeconds(v.Start)
	if err != nil {
		return 0, 0, err
	}

	outTCseconds, err := vscommon.TCToSeconds(v.End)
	if err != nil {
		return 0, 0, err
	}

	// This is basically the offset of the tc that we have to remove from the in and out point
	beginTCseconds, err := vscommon.TCToSeconds(beginTC)
	if err != nil {
		return 0, 0, err
	}

	return inTCseconds - beginTCseconds, outTCseconds - beginTCseconds, nil
}
