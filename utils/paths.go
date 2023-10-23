package utils

import (
	"github.com/ansel1/merry/v2"
	"github.com/orsinium-labs/enum"
	"path/filepath"
	"strings"
)

func GetSiblingFolder(path, folder string) (string, error) {
	newFolder := filepath.Clean(filepath.Join(filepath.Dir(path), "..", folder))
	newFolder, err := filepath.Abs(newFolder)
	if err != nil {
		return "", err
	}
	return newFolder, nil
}

func FixFilename(path string) string {
	filename := filepath.Base(path)
	newFilename := strings.Replace(filepath.Clean(filename), " ", "_", -1)
	newPath := filepath.Join(filepath.Dir(path), newFilename)
	return newPath
}

type Drive enum.Member[string]

var (
	IsilonDrive      = Drive{Value: "isilon"}
	DMZShareDrive    = Drive{Value: "dmzshare"}
	Drives           = enum.New(IsilonDrive, DMZShareDrive)
	ErrDriveNotFound = merry.Sentinel("drive not found")
	ErrPathNotValid  = merry.Sentinel("path not valid")
)

func (d Drive) RcloneName() string {
	switch d {
	case IsilonDrive:
		return "isilon"
	case DMZShareDrive:
		return "dmzshare"
	}
	return ""
}

func (d Drive) RclonePath() string {
	switch d {
	case IsilonDrive:
		return "isilon:isilon"
	case DMZShareDrive:
		return "dmz:dmzshare"
	}
	return ""
}

type Path struct {
	Drive Drive
	Path  string
}

func (p Path) WorkerPath() string {
	switch p.Drive {
	case IsilonDrive:
		return filepath.Join("/mnt/isilon", p.Path)
	case DMZShareDrive:
		return filepath.Join("/mnt/dmzshare", p.Path)
	}
	return ""
}

// RcloneFsRemote returns (fs, remote) for rclone usage
func (p Path) RcloneFsRemote() (string, string) {
	switch p.Drive {
	case IsilonDrive:
		return "isilon:", filepath.Join("isilon", p.Path)
	case DMZShareDrive:
		return "dmz:", filepath.Join("dmzshare", p.Path)
	}
	return "", ""
}

func (p Path) RclonePath() string {
	switch p.Drive {
	case IsilonDrive:
		return filepath.Join("isilon:isilon", p.Path)
	case DMZShareDrive:
		return filepath.Join("dmz:dmzshare", p.Path)
	}
	return ""
}

func (p Path) BatonPath() string {
	switch p.Drive {
	case IsilonDrive:
		return filepath.Join("\\\\10.12.130.61\\isilon", strings.ReplaceAll(p.Path, "/", "\\"))
	}
	return ""
}

func (p Path) FileName() string {
	return filepath.Base(p.Path)
}

func (p Path) Append(path string) Path {
	p.Path = filepath.Join(p.Path, path)
	return p
}

func ParsePath(path string) (Path, error) {
	p := Path{}
	if strings.HasPrefix(path, "/mnt/isilon") {
		p.Drive = IsilonDrive
		p.Path = strings.TrimPrefix(path, "/mnt/isilon/")
		return p, nil
	}
	if strings.HasPrefix(path, "/mnt/dmzshare") {
		p.Drive = DMZShareDrive
		p.Path = strings.TrimPrefix(path, "/mnt/dmzshare/")
		return p, nil
	}
	return p, ErrPathNotValid
}
