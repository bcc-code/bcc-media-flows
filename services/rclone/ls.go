package rclone

import (
	"bytes"
	"encoding/json"
	"net/http"
	"time"
)

type listOpts struct {
	Recurse       bool     `json:"recurse,omitempty"`
	NoModTime     bool     `json:"noModTime,omitempty"`
	ShowEncrypted bool     `json:"showEncrypted,omitempty"`
	ShowOrigIDs   bool     `json:"showOrigIDs,omitempty"`
	ShowHash      bool     `json:"showHash,omitempty"`
	NoMimeType    bool     `json:"noMimeType,omitempty"`
	DirsOnly      bool     `json:"dirsOnly,omitempty"`
	FilesOnly     bool     `json:"filesOnly,omitempty"`
	Metadata      bool     `json:"metadata,omitempty"`
	HashTypes     []string `json:"hashTypes,omitempty"`
}

type ListRequest struct {
	Remote string   `json:"fs"`
	Path   string   `json:"remote"`
	Opt    listOpts `json:"opt"`
}

type ListResponse struct {
	List []RcloneFile `json:"list"`
}

type StatsResponse struct {
	File *RcloneFile `json:"item,omitempty"`
}

type RcloneFile struct {
	Path     string    `json:"Path"`
	Name     string    `json:"Name"`
	Size     int       `json:"Size"`
	MimeType string    `json:"MimeType"`
	ModTime  time.Time `json:"ModTime"`
	IsDir    bool      `json:"IsDir"`
}

func ListFiles(remote, path string) ([]RcloneFile, error) {
	body, err := json.Marshal(ListRequest{
		Remote: remote,
		Path:   path,
		Opt: listOpts{
			Recurse:   false,
			FilesOnly: true,
		}})
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", baseUrl+"/operations/list", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	resp, err := doRequest[ListResponse](req)
	if err != nil {
		return nil, err
	}
	return resp.List, nil
}

func Stat(remote, path string) (*RcloneFile, error) {
	body, err := json.Marshal(ListRequest{
		Remote: remote,
		Path:   path,
		Opt: listOpts{
			Recurse:   false,
			FilesOnly: true,
		}})
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", baseUrl+"/operations/stat", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	resp, err := doRequest[StatsResponse](req)
	if err != nil {
		return nil, err
	}
	return resp.File, nil
}
