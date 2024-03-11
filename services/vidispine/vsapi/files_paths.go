package vsapi

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/bcc-code/bcc-media-flows/environment"
	"github.com/samber/lo"
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

func (c *Client) UpdateFileState(fileID string, fileState FileState) error {
	_, err := c.restyClient.R().
		Put("/file/" + fileID + "/state/" + string(fileState))
	if err != nil {
		return err
	}

	return nil
}

type ListFilesFilter string

const (
	AllFiles          ListFilesFilter = "files"
	AssociatedFiles   ListFilesFilter = "item"
	UnassociatedFiles ListFilesFilter = "noitem"
)

func (c *Client) ListFilesForStorage(
	storageID string,
	rootPath string,
	recursive bool,
	count int,
	offset int,
	filter []ListFilesFilter,
) (*FileSearchResult, error) {
	requestURL, _ := url.Parse(c.baseURL)
	requestURL.Path += "/storage/" + url.PathEscape(storageID) + "/file"
	q := requestURL.Query()
	q.Set("path", rootPath)
	q.Set("recursive", fmt.Sprintf("%t", recursive))
	q.Set("includeItem", "true")
	q.Set("first", fmt.Sprintf("%d", offset))
	q.Set("number", fmt.Sprintf("%d", count))

	if len(filter) > 0 {
		qFilter := lo.Reduce(filter, func(agg string, f ListFilesFilter, _ int) string {
			return agg + string(f) + ","
		}, "")
		q.Set("filter", strings.TrimRight(qFilter, ","))
	}

	requestURL.RawQuery = q.Encode()
	result, err := c.restyClient.R().
		SetResult(&FileSearchResult{}).
		Get(requestURL.String())

	//spew.Dump(string(result.Body()))
	if err != nil {
		return nil, err
	}

	return result.Result().(*FileSearchResult), nil
}

func (c *Client) MoveFile(fileID string, newStorageID string, newName string) (*JobDocument, error) {
	fileID = url.PathEscape(fileID)
	newStorageID = url.PathEscape(newStorageID)

	requestURL, _ := url.Parse(c.baseURL)
	requestURL.Path += fmt.Sprintf("/file/%s/storage/%s", fileID, newStorageID)
	q := requestURL.Query()
	q.Set("useOriginalFilename", "false")
	q.Set("move", "true")
	q.Set("filename", newName)

	requestURL.RawQuery = q.Encode()

	job := &JobDocument{}
	res, err := c.restyClient.R().
		SetResult(job).
		Post(requestURL.String())

	if err != nil {
		return nil, err
	}

	return res.Result().(*JobDocument), nil
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

type FileSearchResult struct {
	Hits  int    `json:"hits,omitempty"`
	Files []File `json:"file,omitempty"`
}

type Component struct {
	ID string `json:"id,omitempty"`
}

type Item struct {
	ID    string  `json:"id,omitempty"`
	Shape []Shape `json:"shape,omitempty"`
}

type Field struct {
	Key   string `json:"key,omitempty"`
	Value string `json:"value,omitempty"`
}

type Metadata struct {
	Field []Field `json:"field,omitempty"`
}
