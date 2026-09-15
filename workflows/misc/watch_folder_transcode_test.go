package miscworkflows

import (
	"encoding/json"
	"testing"

	"github.com/bcc-code/bcc-media-flows/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Every watch folder must be handled: either as a one-activity encode in
// watchFolderEncodes or by a dedicated branch in WatchFolderTranscode.
func TestEveryWatchFolderIsHandled(t *testing.T) {
	branches := map[common.WatchFolder]bool{
		common.FolderTranscribe: true,
		common.FolderHAP50FPS:   true,
	}
	for _, folder := range common.WatchFolders.Members() {
		_, encoded := watchFolderEncodes[folder]
		assert.True(t, encoded || branches[folder], "watch folder %q is not handled", folder.Value)
	}
}

// The input has to keep decoding histories recorded while FolderName was a
// plain string, and reject folder names nothing handles.
func TestWatchFolderTranscodeInput_JSONRoundTrip(t *testing.T) {
	var in WatchFolderTranscodeInput
	require.NoError(t, json.Unmarshal([]byte(`{"Path":"/x/in/a.mov","FolderName":"ProRes444_4K-25FPS"}`), &in))
	assert.Equal(t, common.FolderProRes4444K25FPS, in.FolderName)

	out, err := json.Marshal(in)
	require.NoError(t, err)
	assert.JSONEq(t, `{"Path":"/x/in/a.mov","FolderName":"ProRes444_4K-25FPS"}`, string(out))

	err = json.Unmarshal([]byte(`{"Path":"/x/in/a.mov","FolderName":"DNxHD"}`), &in)
	assert.ErrorIs(t, err, common.ErrUnknownWatchFolder)
}
