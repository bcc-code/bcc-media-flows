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

// RcloneFsRemote returns (fs, remote) for rclone usage
func (d Path) RcloneFsRemote() (string, string) {
	switch d.Drive {
	case IsilonDrive:
		return "isilon:", filepath.Join("isilon", d.Path)
	case DMZShareDrive:
		return "dmz:", filepath.Join("dmzshare", d.Path)
	}
	return "", ""
}

func (d Path) RclonePath() string {
	switch d.Drive {
	case IsilonDrive:
		return filepath.Join("isilon:isilon", d.Path)
	case DMZShareDrive:
		return filepath.Join("dmz:dmzshare", d.Path)
	}
	return ""
}

func (d Path) FileName() string {
	return filepath.Base(d.Path)
}

func (d Path) Append(path string) Path {
	d.Path = filepath.Join(d.Path, path)
	return d
}

func ParsePath(path string) (Path, error) {
	p := Path{}
	if strings.HasPrefix(path, "/mnt/isilon") {
		p.Drive = IsilonDrive
		p.Path = strings.TrimPrefix(path, "/mnt/isilon/")
		return p, nil
	}
	return p, ErrPathNotValid
}
