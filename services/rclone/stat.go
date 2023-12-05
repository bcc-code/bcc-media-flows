package rclone

import (
	"bytes"
	"encoding/json"
	"net/http"
)

type statRequest struct {
	Remote string `json:"fs"`
	Path   string `json:"remote"`
}

type StatResponse struct {
	ID       string `json:"ID"`
	OrigID   string `json:"OrigID"`
	IsBucket bool   `json:"IsBucket"`
	IsDir    bool   `json:"IsDir"`
	MimeType string `json:"MimeType"`
	ModTime  string `json:"ModTime"`
	Name     string `json:"Name"`
	Path     string `json:"Path"`
	Size     int64  `json:"Size"`
	Tier     string `json:"Tier"`
}

func Stat(remote, path string) (*StatResponse, error) {
	body, err := json.Marshal(statRequest{
		Remote: remote,
		Path:   path,
	})
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", baseUrl+"/operations/stat", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	return doRequest[StatResponse](req)
}
