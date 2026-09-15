package directus

import (
	"github.com/bcc-code/bcc-media-flows/internal/enumjson"
	"github.com/orsinium-labs/enum"
)

// ShortStatus is the publication state of a short. Directus's status field
// offers the three states below; flows only ever create drafts, editors
// publish in the CMS.
type ShortStatus enum.Member[string]

var (
	ShortStatusDraft     = ShortStatus{Value: "draft"}
	ShortStatusPublished = ShortStatus{Value: "published"}
	ShortStatusArchived  = ShortStatus{Value: "archived"}
	ShortStatuses        = enum.New(ShortStatusDraft, ShortStatusPublished, ShortStatusArchived)
)

func (s ShortStatus) String() string { return s.Value }

//goland:noinspection GoMixedReceiverTypes
func (s ShortStatus) MarshalJSON() ([]byte, error) { return enumjson.Marshal(s) }

// UnmarshalJSON keeps any state Directus reports, since the CMS owns the list.
//
//goland:noinspection GoMixedReceiverTypes
func (s *ShortStatus) UnmarshalJSON(data []byte) error { return enumjson.UnmarshalOpen(data, s) }

// MediaItemType is the kind of media item a Directus record describes. Flows
// only create shorts; the type is open because the CMS holds other kinds.
type MediaItemType enum.Member[string]

var (
	MediaItemTypeShort = MediaItemType{Value: "short"}
	MediaItemTypes     = enum.New(MediaItemTypeShort)
)

func (t MediaItemType) String() string { return t.Value }

//goland:noinspection GoMixedReceiverTypes
func (t MediaItemType) MarshalJSON() ([]byte, error) { return enumjson.Marshal(t) }

//goland:noinspection GoMixedReceiverTypes
func (t *MediaItemType) UnmarshalJSON(data []byte) error { return enumjson.UnmarshalOpen(data, t) }

// ImageStyle is the slot a styled image fills on a media item.
type ImageStyle enum.Member[string]

var (
	ImageStylePoster   = ImageStyle{Value: "poster"}
	ImageStyleDefault  = ImageStyle{Value: "default"}
	ImageStyleIcon     = ImageStyle{Value: "icon"}
	ImageStyleAlbum    = ImageStyle{Value: "album"}
	ImageStyleFeatured = ImageStyle{Value: "featured"}
	ImageStyles        = enum.New(ImageStylePoster, ImageStyleDefault, ImageStyleIcon, ImageStyleAlbum, ImageStyleFeatured)
)

func (s ImageStyle) String() string { return s.Value }

//goland:noinspection GoMixedReceiverTypes
func (s ImageStyle) MarshalJSON() ([]byte, error) { return enumjson.Marshal(s) }

//goland:noinspection GoMixedReceiverTypes
func (s *ImageStyle) UnmarshalJSON(data []byte) error { return enumjson.UnmarshalOpen(data, s) }
