package rclone

import (
	"encoding/base64"
	"encoding/json"
	"github.com/bcc-code/bcc-media-flows/environment"
	"io"
	"net/http"
	"time"

	"github.com/ansel1/merry/v2"

	"github.com/bcc-code/bcc-media-flows/internal/httpx"
)

var (
	errNon200Status = merry.Sentinel("non-200 status")
)

// requestTimeout bounds one call to the rclone API. Every request in this package
// either submits a job with _async or reads status, so none of them carries a
// transfer: the copy happens inside rclone and is polled through /job/status.
const requestTimeout = 30 * time.Second

// client is the whole package's HTTP client. Every file copy in the tree goes through
// it, so the timeout is what stops an rclone that accepts a connection and then stops
// answering from holding an activity for its entire budget.
var client = &http.Client{Timeout: requestTimeout}

// maxErrorBody bounds how much of a failed response is quoted into the error.
const maxErrorBody = 2048

func doRequest[T any](req *http.Request) (*T, error) {
	if req.Body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Close = true

	cfg := environment.Get()
	basicAuth := base64.StdEncoding.EncodeToString([]byte(cfg.RcloneUsername + ":" + cfg.RclonePassword))
	req.Header.Set("Authorization", "Basic "+basicAuth)

	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	defer func() {
		_ = res.Body.Close()
	}()

	if res.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(res.Body, maxErrorBody))
		return nil, merry.Wrap(errNon200Status,
			merry.WithHTTPCode(res.StatusCode),
			merry.WithMessagef("rclone %s %s returned %s: %s",
				req.Method, req.URL.Path, res.Status, httpx.TruncateBody(body)))
	}

	var response *T
	err = json.NewDecoder(res.Body).Decode(&response)
	if err != nil {
		return nil, err
	}
	return response, nil
}
