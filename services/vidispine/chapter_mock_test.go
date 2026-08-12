package vidispine

import (
	"errors"
	"fmt"
	"testing"

	"github.com/bcc-code/bcc-media-flows/services/vidispine/vsapi"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/bcc-code/bcc-media-flows/services/vidispine/vsmock"
)

// Deterministic coverage for GetChapterMetaForClips using the generated Client
// mock. The integration test in chapter_integration_test.go exercises the same
// function against production data, which makes it useful for spot-checking real
// assets but useless as a regression test: it needs an internal host and its
// expectations depend on metadata nobody controls.

// palTC renders seconds as the frames@PAL timecode Vidispine returns (25 fps).
func palTC(seconds float64) string {
	return fmt.Sprintf("%.0f@PAL", seconds*25)
}

// chapter builds a chapter metadata result spanning the given seconds.
func chapter(title string, startSeconds, endSeconds float64) *vsapi.MetadataResult {
	return &vsapi.MetadataResult{
		Terse: map[string][]*vsapi.MetadataField{
			"title": {
				{
					Value: title,
					Start: palTC(startSeconds),
					End:   palTC(endSeconds),
				},
			},
		},
	}
}

// clipAt builds a clip covering the given seconds of one asset.
func clipAt(vxID string, inSeconds, outSeconds float64) *Clip {
	return &Clip{
		VXID:          vxID,
		VideoFile:     "/dummy/file.mp4",
		InSeconds:     inSeconds,
		OutSeconds:    outSeconds,
		AudioFiles:    map[string]*AudioFile{},
		SubtitleFiles: map[string]string{},
	}
}

// emptyMeta is the asset-level metadata lookup, which GetChapterMetaForClips uses
// only to read startTimeCode. Absent, it falls back to 0.
func emptyMeta() *vsapi.MetadataResult {
	return &vsapi.MetadataResult{Terse: map[string][]*vsapi.MetadataField{}}
}

func TestGetChapterMetaForClips_SingleChapter(t *testing.T) {
	ctrl := gomock.NewController(t)
	vs := vsmock.NewMockClient(ctrl)

	vs.EXPECT().GetMetadata("VX-1").Return(emptyMeta(), nil).Times(1)
	vs.EXPECT().GetChapterMeta("VX-1", 10.0, 70.0).Return(map[string]*vsapi.MetadataResult{
		"Opening": chapter("Opening", 12, 30),
	}, nil).Times(1)

	out, err := GetChapterMetaForClips(vs, []*Clip{clipAt("VX-1", 10, 70)})

	assert.NoError(t, err)
	assert.Len(t, out, 1)
	assert.Equal(t, 10.0, out[0].OriginalStart)
}

// Two annotations that overlap only slightly arrive from Vidispine keyed by title,
// so same-titled chapters collapse before the merge logic is reached. This is the
// case the overlapping integration test covers.
func TestGetChapterMetaForClips_SameTitleCollapsesToOneChapter(t *testing.T) {
	ctrl := gomock.NewController(t)
	vs := vsmock.NewMockClient(ctrl)

	vs.EXPECT().GetMetadata("VX-1").Return(emptyMeta(), nil).Times(1)
	vs.EXPECT().GetChapterMeta("VX-1", 1420.0, 2767.0).Return(map[string]*vsapi.MetadataResult{
		"Sermon": chapter("Sermon", 1425, 2760),
	}, nil).Times(1)

	out, err := GetChapterMetaForClips(vs, []*Clip{clipAt("VX-1", 1420, 2767)})

	assert.NoError(t, err)
	assert.Len(t, out, 1, "one chapter title should yield one chapter")
}

// Distinct titles are kept apart and ordered by where their clip starts.
func TestGetChapterMetaForClips_SortsByClipStart(t *testing.T) {
	ctrl := gomock.NewController(t)
	vs := vsmock.NewMockClient(ctrl)

	vs.EXPECT().GetMetadata("VX-1").Return(emptyMeta(), nil).Times(1)
	vs.EXPECT().GetChapterMeta("VX-1", 500.0, 600.0).Return(map[string]*vsapi.MetadataResult{
		"Later": chapter("Later", 510, 590),
	}, nil).Times(1)
	vs.EXPECT().GetChapterMeta("VX-1", 10.0, 70.0).Return(map[string]*vsapi.MetadataResult{
		"Earlier": chapter("Earlier", 12, 30),
	}, nil).Times(1)

	out, err := GetChapterMetaForClips(vs, []*Clip{
		clipAt("VX-1", 500, 600),
		clipAt("VX-1", 10, 70),
	})

	assert.NoError(t, err)
	assert.Len(t, out, 2)
	assert.Equal(t, 10.0, out[0].OriginalStart, "earlier clip first")
	assert.Equal(t, 500.0, out[1].OriginalStart)
}

// Asset metadata is fetched once per VXID even across several clips.
func TestGetChapterMetaForClips_CachesMetadataPerAsset(t *testing.T) {
	ctrl := gomock.NewController(t)
	vs := vsmock.NewMockClient(ctrl)

	vs.EXPECT().GetMetadata("VX-1").Return(emptyMeta(), nil).Times(1)
	vs.EXPECT().GetChapterMeta("VX-1", gomock.Any(), gomock.Any()).
		Return(map[string]*vsapi.MetadataResult{}, nil).Times(3)

	out, err := GetChapterMetaForClips(vs, []*Clip{
		clipAt("VX-1", 10, 20),
		clipAt("VX-1", 30, 40),
		clipAt("VX-1", 50, 60),
	})

	assert.NoError(t, err)
	assert.Empty(t, out)
}

// startTimeCode offsets the range queried from Vidispine, since clip times are
// relative to the sequence but chapters are stored in media timecode.
func TestGetChapterMetaForClips_AppliesStartTimecodeOffset(t *testing.T) {
	ctrl := gomock.NewController(t)
	vs := vsmock.NewMockClient(ctrl)

	withStartTC := &vsapi.MetadataResult{
		Terse: map[string][]*vsapi.MetadataField{
			"startTimeCode": {{Value: palTC(100)}},
		},
	}

	vs.EXPECT().GetMetadata("VX-1").Return(withStartTC, nil).Times(1)
	// 10 + 100 and 70 + 100.
	vs.EXPECT().GetChapterMeta("VX-1", 110.0, 170.0).Return(map[string]*vsapi.MetadataResult{
		"Opening": chapter("Opening", 112, 130),
	}, nil).Times(1)

	out, err := GetChapterMetaForClips(vs, []*Clip{clipAt("VX-1", 10, 70)})

	assert.NoError(t, err)
	assert.Len(t, out, 1)
}

func TestGetChapterMetaForClips_PropagatesMetadataError(t *testing.T) {
	ctrl := gomock.NewController(t)
	vs := vsmock.NewMockClient(ctrl)

	vs.EXPECT().GetMetadata("VX-1").Return(nil, errors.New("vidispine unreachable")).Times(1)

	out, err := GetChapterMetaForClips(vs, []*Clip{clipAt("VX-1", 10, 70)})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "vidispine unreachable")
	assert.Nil(t, out)
}

func TestGetChapterMetaForClips_PropagatesChapterError(t *testing.T) {
	ctrl := gomock.NewController(t)
	vs := vsmock.NewMockClient(ctrl)

	vs.EXPECT().GetMetadata("VX-1").Return(emptyMeta(), nil).Times(1)
	vs.EXPECT().GetChapterMeta("VX-1", 10.0, 70.0).
		Return(nil, errors.New("chapter lookup failed")).Times(1)

	out, err := GetChapterMetaForClips(vs, []*Clip{clipAt("VX-1", 10, 70)})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "chapter lookup failed")
	assert.Nil(t, out)
}
