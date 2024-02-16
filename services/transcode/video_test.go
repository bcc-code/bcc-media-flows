package transcode

import (
	"encoding/json"
	"testing"

	"github.com/bcc-code/bcc-media-flows/common"
	"github.com/bcc-code/bcc-media-flows/paths"
	"github.com/bcc-code/bcc-media-flows/services/ffmpeg"
	"github.com/davecgh/go-spew/spew"
	"github.com/stretchr/testify/assert"
)

func Test_Video(t *testing.T) {
	p, stop := printProgress()
	defer close(stop)

	wm := paths.MustParse("/mnt/isilon/system/assets/BTV_LOGO_WATERMARK_BUG_GFX_1080.png")
	_, err := VideoH264(common.VideoInput{
		Path:            paths.MustParse("/mnt/isilon/system/tmp/workflows/07c4a523-ee62-481b-b473-80cc82fffb0a/SOTM_7v2123_SEQ.mxf"),
		DestinationPath: paths.MustParse("/mnt/isilon/system/tmp/workflows/07c4a523-ee62-481b-b473-80cc82fffb0a"),
		Bitrate:         "1M",
		Width:           1280,
		Height:          720,
		FrameRate:       25,
		WatermarkPath:   &wm,
	}, p)
	assert.Nil(t, err)
}

var testjson = `{"Title":"ROMR_GANI_E01_HIRO_SEQ","Items":[{"Path":{"Drive":"isilon","Path":"Production/raw/2024/02/12/ROMR_GANI_E01_HIRO_MAS_NOR.mov"},"Start":0,"End":30,"Streams":null}],"OutputDir":{"Drive":"temp","Path":"workflows/1fbe4b81-552a-4339-b9a9-778f34c7f65a"},"WorkDir":{"Drive":"temp","Path":"workflows/1fbe4b81-552a-4339-b9a9-778f34c7f65a"},"Duration":30}`

func Test_MergeVideo(t *testing.T) {
	var mergeInput common.MergeInput

	_ = json.Unmarshal([]byte(testjson), &mergeInput)

	mergeInput.OutputDir = paths.MustParse("/mnt/temp/test")

	_, _ = MergeVideo(mergeInput, func(progress ffmpeg.Progress) {
		spew.Dump(progress)
	})
}
