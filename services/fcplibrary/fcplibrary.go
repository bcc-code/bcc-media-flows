// Package fcplibrary restores a Final Cut Pro library stored by Cantemo Portal's
// project sync as a .fcpbundlezip.
//
// The zip holds the library without its media. portalproject.xml in the library
// root lists each media item by VX-ID with its path inside the library, e.g.
// "4-17-23/Original Media/clip.mxf". The library's event databases refer to media
// by those relative paths, so placing each file at its path makes the library open
// with its media online, without editing the databases.
package fcplibrary

import (
	"archive/zip"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// ArchiveExtension is the extension Cantemo Portal gives a zipped FCP library.
const ArchiveExtension = ".fcpbundlezip"

// PortalProjectFile is the file in the library root that maps VX-IDs to library paths.
const PortalProjectFile = "portalproject.xml"

var (
	ErrNoPortalProject = errors.New("library has no " + PortalProjectFile)
	errUnsafePath      = errors.New("path leaves the library")
)

// lockAndCacheEntries are left behind by the editor's Mac. A copied lock makes Final
// Cut report the library as in use, and the cache link points into their home folder.
var lockAndCacheEntries = []string{".lock-info", ".lock-dir", ".fcpcache"}

// IsArchive reports whether a filename is a zipped FCP library.
func IsArchive(filename string) bool {
	return strings.EqualFold(filepath.Ext(filename), ArchiveExtension)
}

// LibraryName returns the library folder name for an archive, "X.fcpbundlezip" → "X.fcpbundle".
func LibraryName(archiveName string) string {
	base := filepath.Base(archiveName)
	return strings.TrimSuffix(base, filepath.Ext(base)) + ".fcpbundle"
}

// Item is a media item the library expects, with its path relative to the library root.
type Item struct {
	VXID string
	Path string
}

// SkippedItem is a portalproject.xml entry that cannot be placed in the library.
type SkippedItem struct {
	VXID   string
	Path   string
	Reason string
}

// Extract unzips archive into dir, which is created, and removes the lock and cache entries.
// Symlinks in the archive are not restored.
func Extract(archive, dir string) error {
	r, err := zip.OpenReader(archive)
	if err != nil {
		return fmt.Errorf("opening %s: %w", archive, err)
	}
	defer r.Close()

	if err := os.MkdirAll(dir, os.ModePerm); err != nil {
		return err
	}

	for _, f := range r.File {
		target, err := safeJoin(dir, f.Name)
		if err != nil {
			return fmt.Errorf("archive entry %q: %w", f.Name, err)
		}
		mode := f.Mode()
		switch {
		case mode.IsDir():
			if err := os.MkdirAll(target, os.ModePerm); err != nil {
				return err
			}
		case mode&os.ModeSymlink != 0:
			continue
		default:
			if err := extractFile(f, target); err != nil {
				return fmt.Errorf("extracting %q: %w", f.Name, err)
			}
		}
	}

	for _, name := range lockAndCacheEntries {
		if err := os.RemoveAll(filepath.Join(dir, name)); err != nil {
			return fmt.Errorf("removing %s: %w", name, err)
		}
	}
	return nil
}

func extractFile(f *zip.File, target string) error {
	if err := os.MkdirAll(filepath.Dir(target), os.ModePerm); err != nil {
		return err
	}
	src, err := f.Open()
	if err != nil {
		return err
	}
	defer src.Close()

	dst, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.ModePerm)
	if err != nil {
		return err
	}
	if _, err := io.Copy(dst, src); err != nil {
		return errors.Join(err, dst.Close())
	}
	return dst.Close()
}

// safeJoin joins a slash-separated relative path onto dir, refusing paths that leave dir.
func safeJoin(dir, rel string) (string, error) {
	clean := path.Clean("/" + strings.ReplaceAll(rel, "\\", "/"))
	if clean == "/" || strings.HasPrefix(rel, "/") || strings.Contains("/"+rel+"/", "/../") {
		return "", errUnsafePath
	}
	return filepath.Join(dir, filepath.FromSlash(clean[1:])), nil
}

type portalProject struct {
	ID      string `xml:"id"`
	Version struct {
		Items []struct {
			ID   string `xml:"id"`
			Path string `xml:"path"`
		} `xml:"items>item"`
	} `xml:"version"`
}

// ParsePortalProject reads the items of a portalproject.xml.
func ParsePortalProject(r io.Reader) ([]Item, error) {
	var p portalProject
	if err := xml.NewDecoder(r).Decode(&p); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", PortalProjectFile, err)
	}
	items := make([]Item, 0, len(p.Version.Items))
	for _, it := range p.Version.Items {
		items = append(items, Item{VXID: strings.TrimSpace(it.ID), Path: strings.TrimSpace(it.Path)})
	}
	return items, nil
}

// Items reads portalproject.xml from an extracted library and splits its entries into
// those that can be placed in the library and those that cannot. An entry is placed
// only when it has a VX-ID and its path stays in the library and starts with a folder
// the library has, such as an event; Cantemo also lists files from the editor's disk.
func Items(libraryDir string) ([]Item, []SkippedItem, error) {
	f, err := os.Open(filepath.Join(libraryDir, PortalProjectFile))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil, ErrNoPortalProject
	}
	if err != nil {
		return nil, nil, err
	}
	defer f.Close()

	all, err := ParsePortalProject(f)
	if err != nil {
		return nil, nil, err
	}

	var items []Item
	var skipped []SkippedItem
	seen := map[string]bool{}
	for _, it := range all {
		reason := ""
		switch {
		case it.VXID == "" || it.Path == "":
			reason = "missing VX-ID or path"
		case seen[it.Path]:
			reason = "duplicate path"
		default:
			if _, err := safeJoin(libraryDir, it.Path); err != nil {
				reason = err.Error()
			} else if !isDir(filepath.Join(libraryDir, strings.SplitN(it.Path, "/", 2)[0])) {
				reason = "path is not inside a folder of the library"
			}
		}
		if reason != "" {
			skipped = append(skipped, SkippedItem{VXID: it.VXID, Path: it.Path, Reason: reason})
			continue
		}
		seen[it.Path] = true
		items = append(items, it)
	}
	return items, skipped, nil
}

func isDir(p string) bool {
	st, err := os.Stat(p)
	return err == nil && st.IsDir()
}
