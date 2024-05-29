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

		if clip.Get(vscommon.FieldExportAsChapter, "") != "export_as_chapter" {
			continue
		}

		// TODO: @KillerX can you document why we need to do this? e.g. by extracting into a function
		for _, field := range clip.Terse {
			for _, value := range field {
				if valueStart, _ := vscommon.TCToSeconds(value.Start); valueStart < inTc {
					value.Start = fmt.Sprintf("%.0f@PAL", inTc*25)
				}

				if valueEnd, _ := vscommon.TCToSeconds(value.End); valueEnd > outTc {
					value.End = fmt.Sprintf("%.0f@PAL", outTc*25)
				}

			}
		}

		outClips[key] = clip
	}

	return outClips, nil
}
