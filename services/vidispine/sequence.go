package vidispine

import (
	"encoding/xml"
	"fmt"
)

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

func (s *SequenceDocument) ValidateTimeBase() bool {
	// Currently we only support 25fps, so we can just check the denominator
	for _, track := range s.Track {
		for _, segment := range track.Segments {
			if segment.In.TimeBase.Denominator != 25 {
				return false
			}
		}
	}
	return true
}

func (s *SequenceDocument) ToClips(c *Client, audioSource ExportAudioSource) ([]*Clip, error) {
	out := []*Clip{}

	for _, track := range s.Track {
		if track.Audio {
			continue
		}

		for _, segment := range track.Segments {
			clip := &Clip{
				VXID: segment.VXID,
			}

			clip.InSeconds = float64(segment.SourceIn.Samples) / 25.0
			clip.OutSeconds = float64(segment.SourceOut.Samples) / 25.0

			shapes, err := c.GetShapes(segment.VXID)
			if err != nil {
				return nil, err
			}

			shape := shapes.GetShape("original")
			if shape == nil {
				return nil, fmt.Errorf("no original shape found for item %s", segment.VXID)
			}

			clip.VideoFile = shape.GetPath()
			out = append(out, clip)
		}

	}

	return out, nil
}
