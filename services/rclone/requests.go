package rclone

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
)

var (
	username = os.Getenv("RCLONE_USERNAME")
	password = os.Getenv("RCLONE_PASSWORD")
)

func doRequest[T any](req *http.Request) (*T, error) {
	if req.Body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Close = true

	basicAuth := base64.StdEncoding.EncodeToString([]byte(username + ":" + password))
	req.Header.Set("Authorization", "Basic "+basicAuth)

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = res.Body.Close()
	}()

	if res.StatusCode != 200 {
		return nil, fmt.Errorf("rclone returned %d", res.StatusCode)
	}

	var response *T
	err = json.NewDecoder(res.Body).Decode(&response)
	if err != nil {
		return nil, err
	}
	return response, nil
}
