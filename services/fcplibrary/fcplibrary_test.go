package fcplibrary

import (
	"archive/zip"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const samplePortalProject = `<?xml version="1.0" encoding="utf-8" standalone="yes"?>
<PortalNleProject>
    <id>VX-454970</id>
    <nle_project_id></nle_project_id>
    <version>
        <id>3</id>
        <items>
            <item><id>VX-1</id><path>4-17-23/Original Media/clip one.mxf</path></item>
            <item><id>VX-2</id><path>OUT/Original Media/song.wav</path></item>
            <item><id>VX-3</id><path>4-17-23/Transcoded Media/Proxy Media/proxy.mp4</path></item>
            <item><id>VX-1</id><path>4-17-23/Original Media/clip one.mxf</path></item>
            <item><id>VX-4</id><path>Users/editor/project.fcpxmld/Contents/a.plist</path></item>
            <item><id>VX-5</id><path>../escape.mov</path></item>
        </items>
    </version>
</PortalNleProject>`

type entry struct {
	name    string
	body    string
	symlink bool
}

func writeZip(t *testing.T, entries []entry) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "LIB.fcpbundlezip")
	f, err := os.Create(p)
	require.NoError(t, err)
	w := zip.NewWriter(f)
	for _, e := range entries {
		h := &zip.FileHeader{Name: e.name, Method: zip.Deflate}
		if strings.HasSuffix(e.name, "/") {
			h.SetMode(os.ModeDir | 0o755)
		} else if e.symlink {
			h.SetMode(os.ModeSymlink | 0o777)
		}
		fw, err := w.CreateHeader(h)
		require.NoError(t, err)
		if !strings.HasSuffix(e.name, "/") {
			_, err = fw.Write([]byte(e.body))
			require.NoError(t, err)
		}
	}
	require.NoError(t, w.Close())
	require.NoError(t, f.Close())
	return p
}

func TestNames(t *testing.T) {
	assert.True(t, IsArchive("/x/BIBX_KS23_V4.fcpbundlezip"))
	assert.True(t, IsArchive("A.FCPBUNDLEZIP"))
	assert.False(t, IsArchive("a.fcpxml"))
	assert.Equal(t, "BIBX_KS23_V4.fcpbundle", LibraryName("/x/BIBX_KS23_V4.fcpbundlezip"))
}

func TestExtractRestoresLibraryAndDropsLocks(t *testing.T) {
	archive := writeZip(t, []entry{
		{name: "Settings.plist", body: "settings"},
		{name: "4-17-23/"},
		{name: "4-17-23/CurrentVersion.fcpevent", body: "db"},
		{name: "OUT/"},
		{name: PortalProjectFile, body: samplePortalProject},
		{name: ".lock-info", body: "lock"},
		{name: ".lock-dir/"},
		{name: ".fcpcache", body: "/Users/editor/Cache.fcpcache", symlink: true},
	})
	dir := filepath.Join(t.TempDir(), "LIB.fcpbundle")

	require.NoError(t, Extract(archive, dir))

	b, err := os.ReadFile(filepath.Join(dir, "4-17-23", "CurrentVersion.fcpevent"))
	require.NoError(t, err)
	assert.Equal(t, "db", string(b))
	for _, gone := range []string{".lock-info", ".lock-dir", ".fcpcache"} {
		_, err := os.Lstat(filepath.Join(dir, gone))
		assert.ErrorIs(t, err, os.ErrNotExist, gone)
	}

	items, skipped, err := Items(dir)
	require.NoError(t, err)
	assert.Equal(t, []Item{
		{VXID: "VX-1", Path: "4-17-23/Original Media/clip one.mxf"},
		{VXID: "VX-2", Path: "OUT/Original Media/song.wav"},
		{VXID: "VX-3", Path: "4-17-23/Transcoded Media/Proxy Media/proxy.mp4"},
	}, items)
	if assert.Len(t, skipped, 3) {
		assert.Equal(t, "duplicate path", skipped[0].Reason)
		assert.Equal(t, "VX-4", skipped[1].VXID, "files from the editor's disk are not placed in the library")
		assert.Equal(t, "VX-5", skipped[2].VXID)
	}
}

func TestExtractRefusesEntriesOutsideTheLibrary(t *testing.T) {
	archive := writeZip(t, []entry{{name: "../evil.txt", body: "x"}})
	err := Extract(archive, filepath.Join(t.TempDir(), "LIB.fcpbundle"))
	assert.ErrorIs(t, err, errUnsafePath)
}

func TestItemsWithoutPortalProject(t *testing.T) {
	_, _, err := Items(t.TempDir())
	assert.ErrorIs(t, err, ErrNoPortalProject)
}
