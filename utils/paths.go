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
	TempDrive        = Drive{Value: "temp"}
	DMZShareDrive    = Drive{Value: "dmzshare"}
	Drives           = enum.New(IsilonDrive, DMZShareDrive, TempDrive)
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
	return filepath.Join(drivePrefixes[p.Drive].Rclone, p.Path)
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

type prefix struct {
	Linux  string
	Client string
	Rclone string
}

var drivePrefixes = map[Drive]prefix{
	IsilonDrive:   {"/mnt/isilon/", GetIsilonPrefix(), "isilon:isilon/"},
	DMZShareDrive: {"/mnt/dmzshare/", "/mnt/dmzshare/", "dmz:dmzshare/"},
	TempDrive:     {"/mnt/temp/", GetTempMountPrefix(), "isilon:temp/"},
}

func ParsePath(path string) (Path, error) {
	for drive, ps := range drivePrefixes {
		prefixes := []string{ps.Linux, ps.Client, ps.Rclone}
		for _, p := range prefixes {
			if strings.HasPrefix(path, p) {
				return Path{
					Drive: drive,
					Path:  strings.TrimPrefix(path, p),
				}, nil
			}
		}
	}
	return Path{}, ErrPathNotValid
}
