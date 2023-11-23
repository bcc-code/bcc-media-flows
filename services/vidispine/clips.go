package vidispine

import (
	"fmt"

	"github.com/bcc-code/bccm-flows/services/vidispine/vsapi"
)

func SeqToClips(client VSClient, seq *vsapi.SequenceDocument, audioSource ExportAudioSource) ([]*Clip, error) {
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
