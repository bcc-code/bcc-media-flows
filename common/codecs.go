package common

import (
	"errors"

	"github.com/bcc-code/bcc-media-flows/internal/enumjson"
	"github.com/orsinium-labs/enum"
)

// WatchFolder names a subfolder of the transcode root that the file watcher
// monitors. A file dropped in `<root>/<WatchFolder>/in/` is transcoded (or
// transcribed) according to the folder it landed in.
type WatchFolder enum.Member[string]

var (
	FolderProRes422HQHD          = WatchFolder{Value: "ProRes422HQ_HD"}
	FolderProRes422HQNative      = WatchFolder{Value: "ProRes422HQ_Native"}
	FolderProRes422HQNative25FPS = WatchFolder{Value: "ProRes422HQ_Native_25FPS"}
	FolderProRes4444K25FPS       = WatchFolder{Value: "ProRes444_4K-25FPS"}
	FolderAVCIntra100HD          = WatchFolder{Value: "AVCintra100_HD"}
	FolderXDCAMHD422             = WatchFolder{Value: "XDCAMHD422"}
	FolderTranscribe             = WatchFolder{Value: "Transcribe"}
	FolderHAP50FPS               = WatchFolder{Value: "HAP_50FPS"}
	WatchFolders                 = enum.New(
		FolderProRes422HQHD,
		FolderProRes422HQNative,
		FolderProRes422HQNative25FPS,
		FolderProRes4444K25FPS,
		FolderAVCIntra100HD,
		FolderXDCAMHD422,
		FolderTranscribe,
		FolderHAP50FPS,
	)
	ErrUnknownWatchFolder = errors.New("unknown watch folder")
)

func (f WatchFolder) String() string {
	return f.Value
}

// MarshalJSON writes the bare folder name, which is what workflow histories
// recorded while FolderName was a plain string contain.
//
//goland:noinspection GoMixedReceiverTypes
func (f WatchFolder) MarshalJSON() ([]byte, error) {
	return enumjson.Marshal(f)
}

//goland:noinspection GoMixedReceiverTypes
func (f *WatchFolder) UnmarshalJSON(data []byte) error {
	return enumjson.UnmarshalStrict(data, WatchFolders, f, ErrUnknownWatchFolder)
}
