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
		duration := 0.0

		for _, field := range clip.Terse {
			for _, value := range field {
				duration = trimTimecodesToBeWithinRange(value, inTc, outTc)
			}
		}

		if duration < 10.0 {
			// If the trimmed chapter duration is < 10s we don't consider it a valid chapter
			continue
		}

		outClips[key] = clip
	}

	return outClips, nil
}

// trimTimecodesToBeWithinRange ensures that the timecodes are within the clip range
func trimTimecodesToBeWithinRange(value *MetadataField, inTc, outTc float64) float64 {
	valueStart, _ := vscommon.TCToSeconds(value.Start)
	if valueStart < inTc {
		value.Start = fmt.Sprintf("%.0f@PAL", inTc*25)
	}

	valueEnd, _ := vscommon.TCToSeconds(value.End)
	if valueEnd > outTc {
		value.End = fmt.Sprintf("%.0f@PAL", outTc*25)
	}

	return valueEnd - valueStart
}
