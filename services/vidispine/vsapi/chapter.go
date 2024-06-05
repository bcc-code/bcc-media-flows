package vsapi

import (
	"fmt"

	"github.com/bcc-code/bcc-media-flows/services/vidispine/vscommon"
)

func (c *Client) GetChapterMeta(itemVXID string, inTc, outTc float64) (map[string]*MetadataResult, error) {
	metaResult, err := c.GetMetadataAdvanced(GetMetadataAdvancedParams{
		ItemID: itemVXID,
		Group:  "Subclips",
		InTC:   inTc,
		OutTC:  outTc,
	})
	if err != nil {
		return nil, err
	}

	clips := metaResult.SplitByClips()
	outClips := map[string]*MetadataResult{}

	for key, clip := range clips {

		for _, field := range clip.Terse {
			for _, value := range field {
				trimTimecodesToBeWithinRange(value, inTc, outTc)
			}
		}

		outClips[key] = clip
	}

	return outClips, nil
}

// trimTimecodesToBeWithinRange ensures that the timecodes are within the clip range
func trimTimecodesToBeWithinRange(value *MetadataField, inTc, outTc float64) {
	if valueStart, _ := vscommon.TCToSeconds(value.Start); valueStart < inTc {
		value.Start = fmt.Sprintf("%.0f@PAL", inTc*25)
	}

	if valueEnd, _ := vscommon.TCToSeconds(value.End); valueEnd > outTc {
		value.End = fmt.Sprintf("%.0f@PAL", outTc*25)
	}
}
