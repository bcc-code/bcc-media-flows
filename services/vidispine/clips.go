package vidispine

import (
	"fmt"

	"github.com/bcc-code/bcc-media-flows/services/vidispine/vsapi"
	"github.com/bcc-code/bcc-media-flows/services/vidispine/vscommon"
)

func SeqToClips(client Client, seq *vsapi.SequenceDocument) ([]*Clip, error) {
	out := []*Clip{}

	for _, track := range seq.Track {
		if track.Audio {
			continue
		}

		for _, segment := range track.Segments {
			clip := &Clip{
				VXID: segment.VXID,
			}

			clip.InSeconds = float64(segment.SourceIn.Samples) / float64(segment.SourceIn.TimeBase.Denominator)
			clip.OutSeconds = float64(segment.SourceOut.Samples) / float64(segment.SourceOut.TimeBase.Denominator)

			clip.SequenceIn = float64(segment.In.Samples) / float64(segment.In.TimeBase.Denominator)
			clip.SequenceOut = float64(segment.Out.Samples) / float64(segment.Out.TimeBase.Denominator)

			shapes, err := client.GetShapes(segment.VXID)
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

// ClipsFromMeta returns a list of clips based off a metadata result
//
// If subclipTitle is provided, it will return a single clip for that subclip
func ClipsFromMeta(client Client, vxID string, meta *vsapi.MetadataResult, subclipTitle string) ([]*Clip, error) {
	metaClips := meta.SplitByClips()
	originalClipMeta := metaClips[vsapi.OriginalClip]

	isSequence := meta.Get(vscommon.FieldSequenceSize, "0") != "0"

	if isSequence {
		seq, err := client.GetSequence(vxID)
		if err != nil {
			return nil, err
		}
		return SeqToClips(client, seq)
	}

	var clips []*Clip
	if subclipTitle != "" {
		clip, err := getClipForSubclip(client, vxID, subclipTitle, originalClipMeta, metaClips)
		if err != nil {
			return nil, err
		}
		clips = append(clips, clip)
	} else {
		clip, err := getClipForAsset(client, vxID, originalClipMeta, metaClips)
		if err != nil {
			return nil, err
		}
		clips = append(clips, clip)
	}

	return clips, nil
}
