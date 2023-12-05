package paths

import (
	"encoding/json"
	"path/filepath"
	"strings"

	"github.com/ansel1/merry/v2"
	"github.com/bcc-code/bccm-flows/environment"
	"github.com/orsinium-labs/enum"
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

//goland:noinspection GoMixedReceiverTypes
func (d Drive) MarshalJSON() ([]byte, error) {
	return json.Marshal(d.Value)
}

//goland:noinspection GoMixedReceiverTypes
func (d *Drive) UnmarshalJSON(value []byte) error {
	var stringValue string
	err := json.Unmarshal(value, &stringValue)
	if err != nil {
		return err
	}
	drive := Drives.Parse(stringValue)
	if drive == nil {
		return ErrDriveNotFound
	}
	*d = *drive
	return nil
}

var (
	IsilonDrive      = Drive{Value: "isilon"}
	TempDrive        = Drive{Value: "temp"}
	DMZShareDrive    = Drive{Value: "dmzshare"}
	BrunstadDrive    = Drive{Value: "brunstad"}
	AssetIngestDrive = Drive{Value: "asset_ingest"}
	Drives           = enum.New(IsilonDrive, DMZShareDrive, TempDrive, AssetIngestDrive, BrunstadDrive)
	ErrDriveNotFound = merry.Sentinel("drive not found")
	ErrPathNotValid  = merry.Sentinel("path not valid")
)

//goland:noinspection GoMixedReceiverTypes
func (d Drive) RcloneName() string {
	switch d {
	case IsilonDrive:
		return "isilon"
	case DMZShareDrive:
		return "dmzshare"
	case BrunstadDrive:
		return "brunstad"
	}
	return ""
}

//goland:noinspection GoMixedReceiverTypes
func (d Drive) RclonePath() string {
	switch d {
	case IsilonDrive:
		return "isilon:isilon"
	case DMZShareDrive:
		return "dmz:dmzshare"
	case AssetIngestDrive:
		return "s3prod:vod-asset-ingest-prod"
	case BrunstadDrive:
		return "brunstad:"
	}
	return ""
}

type Path struct {
	Drive Drive
	Path  string
}

func (p Path) Dir() Path {
	return Path{
		Drive: p.Drive,
		Path:  filepath.Dir(p.Path),
	}
}

// Local returns the path in a local unix style path.
func (p Path) Local() string {
	return filepath.Join(drivePrefixes[p.Drive].Client, p.Path)
}

// Ext returns the file extension
func (p Path) Ext() string {
	return filepath.Ext(p.Path)
}

// RcloneFsRemote returns (fs, remote) for rclone usage
func (p Path) RcloneFsRemote() (string, string) {
	switch p.Drive {
	case TempDrive:
		return "isilon:", filepath.Join("temp", p.Path)
	case IsilonDrive:
		return "isilon:", filepath.Join("isilon", p.Path)
	case DMZShareDrive:
		return "dmz:", filepath.Join("dmzshare", p.Path)
	case AssetIngestDrive:
		return "s3prod:", filepath.Join("vod-asset-ingest-prod", p.Path)
	case BrunstadDrive:
		return "brunstad:/", p.Path
	}
	return "", ""
}

func (p Path) Rclone() string {
	return filepath.Join(drivePrefixes[p.Drive].Rclone, p.Path)
}

func (p Path) Baton() string {
	switch p.Drive {
	case IsilonDrive:
		return filepath.Join("\\\\10.12.130.61\\isilon", strings.ReplaceAll(p.Path, "/", "\\"))
	}
	return ""
}

func (p Path) Base() string {
	return filepath.Base(p.Path)
}

func (p Path) Append(path string) Path {
	return Path{
		Drive: p.Drive,
		Path:  filepath.Clean(filepath.Join(p.Path, path)),
	}
}

type prefix struct {
	Linux  string
	Client string
	Rclone string
}

var drivePrefixes = map[Drive]prefix{
	IsilonDrive:      {"/mnt/isilon/", environment.GetIsilonPrefix(), "isilon:isilon/"},
	DMZShareDrive:    {"/mnt/dmzshare/", "/mnt/dmzshare/", "dmz:dmzshare/"},
	TempDrive:        {"/mnt/temp/", environment.GetTempMountPrefix(), "isilon:temp/"},
	AssetIngestDrive: {"/dev/null/", "/dev/null/", "s3prod:vod-asset-ingest-prod/"},
	BrunstadDrive:    {"/dev/null/", "/dev/null/", "brunstad:/"},
}

func Parse(path string) (Path, error) {
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

func MustParse(path string) Path {
	p, err := Parse(path)
	if err != nil {
		panic(err)
	}
	return p
}

func New(drive Drive, path string) Path {
	return Path{
		Drive: drive,
		Path:  path,
	}
}
