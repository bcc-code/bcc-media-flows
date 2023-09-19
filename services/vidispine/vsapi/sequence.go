package vsapi

import (
	"encoding/xml"
)

func (c *Client) GetSequence(sequenceID string) (*SequenceDocument, error) {
	result, err := c.restyClient.R().
		SetResult(&SequenceDocument{}).
		Get("/item/" + sequenceID + "/sequence/vidispine")
	if err != nil {
		return nil, err
	}
	return result.Result().(*SequenceDocument), nil
}

type TimeBase struct {
	Numerator   int `xml:"numerator"`
	Denominator int `xml:"denominator"`
}

type TimePoint struct {
	Samples  int      `xml:"samples"`
	TimeBase TimeBase `xml:"timeBase"`
}

type Segment struct {
	VXID        string    `xml:"item"`
	SourceTrack int       `xml:"sourceTrack"`
	In          TimePoint `xml:"in"`
	Out         TimePoint `xml:"out"`
	SourceIn    TimePoint `xml:"sourceIn"`
	SourceOut   TimePoint `xml:"sourceOut"`
}

type Track struct {
	Segments []Segment `xml:"segment"`
	Audio    bool      `xml:"audio"`
}

type SequenceDocument struct {
	XMLName xml.Name `xml:"SequenceDocument"`
	ID      string   `xml:"id"`
	Track   []Track  `xml:"track"`
}
