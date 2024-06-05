package vsapi

import (
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

type GetMetadataAdvancedParams struct {
	ItemID string
	Group  string
	InTC   float64
	OutTC  float64
}

func (c *Client) GetMetadataAdvanced(params GetMetadataAdvancedParams) (*MetadataResult, error) {
	inString := fmt.Sprintf("%.2f", params.InTC)
	outString := fmt.Sprintf("%.2f", params.OutTC)
	url := fmt.Sprintf("%s/item/%s?content=metadata&terse=true&sampleRate=PAL&interval=%s-%s&group=%s", c.baseURL, params.ItemID, inString, outString, params.Group)

	resp, err := c.restyClient.R().
		SetResult(&MetadataResult{}).
		Get(url)

	if err != nil {
		return nil, err
	}

	return resp.Result().(*MetadataResult), nil
}

type ItemMetadataFieldParams struct {
	ItemID  string
	GroupID string
	StartTC string
	EndTC   string
	Key     string
	Value   string
}

func (c *Client) SetItemMetadataField(params ItemMetadataFieldParams) error {
	requestURL, _ := url.Parse(c.baseURL)
	requestURL.Path += fmt.Sprintf("/item/%s/metadata", url.PathEscape(params.ItemID))
	q := requestURL.Query()
	requestURL.RawQuery = q.Encode()

	body, err := createSetItemMetadataFieldXml(
		xmlSetItemMetadataFieldParams{
			StartTC: params.StartTC,
			EndTC:   params.EndTC,
			GroupID: params.GroupID,
			Key:     params.Key,
			Value:   params.Value,
			Add:     false,
		},
	)
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

func (c *Client) AddToItemMetadataField(params ItemMetadataFieldParams) error {
	requestURL, _ := url.Parse(c.baseURL)
	requestURL.Path += fmt.Sprintf("/item/%s/metadata", url.PathEscape(params.ItemID))
	q := requestURL.Query()
	requestURL.RawQuery = q.Encode()

	body, err := createSetItemMetadataFieldXml(
		xmlSetItemMetadataFieldParams{
			StartTC: params.StartTC,
			EndTC:   params.EndTC,
			GroupID: params.GroupID,
			Key:     params.Key,
			Value:   params.Value,
			Add:     true,
		},
	)
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
