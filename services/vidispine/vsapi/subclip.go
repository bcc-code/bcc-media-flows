package vsapi

import (
	"fmt"

	"github.com/bcc-code/bcc-media-flows/services/vidispine/vscommon"
)

const (
	// This is an arbitrary value, but it should be unique to prvent collisions with real titles
	OriginalClip = "311a21f7-c07d-4fb6-b34b-fe4125869402"
	MinusInf     = "-INF"
	PlusInf      = "+INF"
)

// SplitByClips will split the metadata into clips based on how vidispine handles subclips
// That is: All metadata on one asset that has the identical start and end timecode belongs to the same subclip
// Metadata that applies to the whole underlying asset is denoted by `-INF` and `+INF` timecode.
//
// The result is a map with the key being either:
// - OriginalClip if the metadata is for the full clip (+INF to -INF)
// - The title of the clip
// - A concatenation of the start and end timecode (if no title exists)
func (meta *MetadataResult) SplitByClips() map[string]*MetadataResult {
	temp := map[string]*MetadataResult{}

	// The metadata is roughtly in form:
	// [
	// 	{
	// 		"start": "TC",
	// 		"end": "TC",
	// 		"uuid": "UUID",
	// 		"value": "VALUE"
	// 	}
	// ]

	// We want to split it into:
	// {
	// 	"TITLE": {
	// 		"start": "TC",
	// 		"end": "TC",
	// 		"uuid": "UUID",
	// 		"value": "VALUE"
	// 	}, ... <more clips>
	// }

	// So first we need to split by the timestam (as that seems to be the only indicator of what
	// clip the metadata belongs together). This is done here using the start and end timecode as the key
	for fieldKey, val := range meta.Terse {
		if len(val) == 0 {
			continue
		}

		for _, innerVal := range val {
			tempKey := fmt.Sprintf("%s-%s", innerVal.Start, innerVal.End)
			if _, ok := temp[tempKey]; !ok {
				temp[tempKey] = &MetadataResult{
					Terse: map[string][]*MetadataField{},
				}
			}

			temp[tempKey].Terse[fieldKey] = append(temp[tempKey].Terse[fieldKey], innerVal)

		}
	}

	// Now that we have the metadata split by clip, we need swap the key to be the title of the clip
	// in order to be able to refer to it *somehow*.
	out := map[string]*MetadataResult{}
	for key, val := range temp {
		if key == "-INF-+INF" {
			key = OriginalClip
		} else {
			key = val.Get(vscommon.FieldTitle, key)
		}

		out[key] = val
		out[key].ID = key
	}

	return out
}
