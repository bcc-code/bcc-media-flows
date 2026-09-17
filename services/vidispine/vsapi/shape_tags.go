package vsapi

import (
	"github.com/bcc-code/bcc-media-flows/internal/enumjson"
	"github.com/orsinium-labs/enum"
)

// ShapeTag names a Vidispine shape-tag. The members below are the fixed tags
// this codebase reads or writes; Vidispine also carries per-language tags
// (see SubtitleShapeTag and languages' MBPreviewTag), so the set is open and
// an unlisted tag still decodes.
type ShapeTag enum.Member[string]

var (
	ShapeTagOriginal               = ShapeTag{Value: "original"}
	ShapeTagLowres                 = ShapeTag{Value: "lowres"}
	ShapeTagLowresWatermarked      = ShapeTag{Value: "lowres_watermarked"}
	ShapeTagLowAudio               = ShapeTag{Value: "lowaudio"}
	ShapeTagTranscriptionJSON      = ShapeTag{Value: "transcription_json"}
	ShapeTagTranscribedSubtitleSRT = ShapeTag{Value: "Transcribed_Subtitle_SRT"}
	ShapeTags                      = enum.New(
		ShapeTagOriginal,
		ShapeTagLowres,
		ShapeTagLowresWatermarked,
		ShapeTagLowAudio,
		ShapeTagTranscriptionJSON,
		ShapeTagTranscribedSubtitleSRT,
	)
)

// SubtitleShapeTag is the tag of the SRT subtitle shape for an ISO-639-2
// language code, e.g. "sub_nor_srt".
func SubtitleShapeTag(lang string) ShapeTag {
	return ShapeTag{Value: "sub_" + lang + "_srt"}
}

func (t ShapeTag) String() string {
	return t.Value
}

//goland:noinspection GoMixedReceiverTypes
func (t ShapeTag) MarshalJSON() ([]byte, error) {
	return enumjson.Marshal(t)
}

//goland:noinspection GoMixedReceiverTypes
func (t *ShapeTag) UnmarshalJSON(data []byte) error {
	return enumjson.UnmarshalOpen(data, t)
}
