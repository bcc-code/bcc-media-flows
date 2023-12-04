package vsapi

import (
	"fmt"
	"github.com/bcc-code/bccm-flows/environment"
	"net/url"
	"strings"
)

const DefaultStorageID = "VX-42"

func (c *Client) GetAbsoluteStoragePath(storageID string) (string, error) {
	result, err := c.restyClient.R().
		SetResult(&StorageResult{}).
		Get("/storage/" + storageID + "/method")

	if err != nil {
		return "", err
	}
	for _, m := range result.Result().(*StorageResult).Methods {
		if strings.HasPrefix(m.URI, "file://") {
			path := strings.TrimPrefix(m.URI, "file://")
			return environment.IsilonPathFix(path), nil
		}
	}

	return "", fmt.Errorf("No local storage found for storage ID %s", storageID)
}

func (c *Client) RegisterFile(filePath string, fileState FileState) (string, error) {

	// We need the absolute path to the storage in order to remove it from the file path
	// Yeah, briliant design. I know.
	storagePath, err := c.GetAbsoluteStoragePath(DefaultStorageID)
	if err != nil {
		return "", err
	}

	// Remove the storage path from the file path
	filePath = strings.TrimPrefix(filePath, storagePath)

	// Do it in a slightly more complicated way, but make sure everything is encoded properly.
	requestURL, _ := url.Parse(c.baseURL)
	requestURL.Path += "/storage/" + url.PathEscape(DefaultStorageID) + "/file"
	q := requestURL.Query()
	q.Set("path", filePath)
	q.Set("createOnly", "false")
	q.Set("state", string(fileState))
	requestURL.RawQuery = q.Encode()

	result, err := c.restyClient.R().
		SetResult(&IDOnlyResult{}).
		Post(requestURL.String())
	if err != nil {
		return "", err
	}

	return result.Result().(*IDOnlyResult).VXID, nil
}

//// SUPPORTING TYPES /////

type StorageResult struct {
	Methods []StorageMethod `json:"method"`
}

type StorageMethod struct {
	VXID           string `json:"id"`
	URI            string `json:"uri"`
	Read           bool   `json:"read"`
	Write          bool   `json:"write"`
	Browse         bool   `json:"browse"`
	LastSuccess    string `json:"lastSuccess"`
	LastFailure    string `json:"lastFailure"`
	FailureMessage string `json:"failureMessage"`
	Type           string `json:"type"`
}
