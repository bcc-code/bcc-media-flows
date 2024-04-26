package rclone

import (
	"encoding/base64"
	"encoding/json"
	"github.com/ansel1/merry/v2"
	"net/http"
	"os"
)

var (
	username = os.Getenv("RCLONE_USERNAME")
	password = os.Getenv("RCLONE_PASSWORD")
)

var (
	errNon200Status = merry.Sentinel("non-200 status")
)

func doRequest[T any](req *http.Request) (*T, error) {
	if req.Body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Close = true

	basicAuth := base64.StdEncoding.EncodeToString([]byte(username + ":" + password))
	req.Header.Set("Authorization", "Basic "+basicAuth)

	tryCount := 1

	var res *http.Response

	for tryCount <= 5 {
		resInner, err := http.DefaultClient.Do(req)
		if err != nil {
			return nil, err
		}

		if resInner.StatusCode != 200 {
			tryCount++
			continue
		}

		res = resInner
		break
	}

	if res.StatusCode != 200 {
		return nil, merry.Wrap(errNon200Status, merry.WithHTTPCode(res.StatusCode))
	}

	defer func() {
		_ = res.Body.Close()
	}()

	var response *T
	err := json.NewDecoder(res.Body).Decode(&response)
	if err != nil {
		return nil, err
	}
	return response, nil
}
