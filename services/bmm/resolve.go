package bmm

import (
	"context"
	"fmt"
	"net/http"
	"time"
)

// ResolveFileURL issues a HEAD request to rawURL, follows redirects, and
// returns the final URL with userinfo stripped. If the server rejects HEAD
// (405/501), it retries with GET. The returned URL is safe to embed in
// workflow input — credentials supplied in rawURL are never carried into it.
//
// If no redirect occurred, the returned URL still points at the original host
// (with userinfo removed). Callers can compare against rawURL to detect that.
func ResolveFileURL(ctx context.Context, rawURL string) (string, error) {
	client := &http.Client{Timeout: 30 * time.Second}

	resp, err := doResolve(ctx, client, http.MethodHead, rawURL)
	if err != nil {
		return "", err
	}
	resp.Body.Close()

	if resp.StatusCode == http.StatusMethodNotAllowed || resp.StatusCode == http.StatusNotImplemented {
		resp, err = doResolve(ctx, client, http.MethodGet, rawURL)
		if err != nil {
			return "", err
		}
		resp.Body.Close()
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("resolve %s: server returned %s", rawURL, resp.Status)
	}

	final := *resp.Request.URL
	final.User = nil
	return final.String(), nil
}

func doResolve(ctx context.Context, client *http.Client, method, rawURL string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, method, rawURL, nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	return resp, nil
}
