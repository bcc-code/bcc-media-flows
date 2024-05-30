package vsapi

import (
	"fmt"

	"github.com/bcc-code/bcc-media-flows/services/vidispine/vscommon"
)

func (c *Client) GetChapterMeta(itemVXID string, inTc, outTc float64) (map[string]*MetadataResult, error) {
	inString := fmt.Sprintf("%.2f", inTc)
	outString := fmt.Sprintf("%.2f", outTc)

	url := fmt.Sprintf("%s/item/%s?content=metadata&terse=true&sampleRate=PAL&interval=%s-%s&group=Subclips", c.baseURL, itemVXID, inString, outString)

	resp, err := c.restyClient.R().
		SetResult(&MetadataResult{}).
		Get(url)

	if err != nil {
		return nil, err
	}

	metaResult := resp.Result().(*MetadataResult)

	clips := metaResult.SplitByClips()
	outClips := map[string]*MetadataResult{}
	
	for key, clip := range clips {

		if clip.Get(vscommon.FieldExportAsChapter, "") != "export_as_chapter" {
			continue
		}

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
