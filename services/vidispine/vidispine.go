package vidispine

import (
	"bytes"
	"fmt"
	"net/url"
	"strings"

	"github.com/go-resty/resty/v2"
)

type itemContentParam string

const DEFAULT_STORAGE_ID = "VX-42"

const (
	ITEM_CONTENT_PARAM_METADATA itemContentParam = "metadata"
	ITEM_CONTENT_PARAM_SHAPE    itemContentParam = "shape"
)

type Client struct {
	baseURL     string
	username    string
	password    string
	restyClient *resty.Client
}

func NewClient(baseURL string, username string, password string) *Client {
	client := resty.New()
	client.SetBasicAuth(username, password)
	client.SetHostURL(baseURL)
	client.SetHeader("accept", "application/json")
	client.SetDisableWarn(true)

	return &Client{
		baseURL:     baseURL,
		username:    username,
		password:    password,
		restyClient: client,
	}
}

func (c *Client) GetMetadata(vsID string) (*MetadataResult, error) {
	url := c.baseURL + "/item/" + vsID + "?content=metadata&terse=true"

	resp, err := c.restyClient.R().
		SetResult(&MetadataResult{}).
		Get(url)

	if err != nil {
		return nil, err
	}

	return resp.Result().(*MetadataResult), nil
}

func (c *Client) GetShapes(vsID string) (*ShapeResult, error) {
	url := c.baseURL + "/item/" + vsID + "?content=shape&terse=true"

	resp, err := c.restyClient.R().
		SetResult(&ShapeResult{}).
		Get(url)

	if err != nil {
		return nil, err
	}

	return resp.Result().(*ShapeResult), nil
}

type placeholderType string

const (
	PLACEHOLDER_TYPE_MASTER      placeholderType = "master"
	PLACEHOLDER_TYPE_RAWMATERIAL placeholderType = "raw"
)

type PlacholderTplData struct {
	Title string
	Email string
}

type IDOnlyResult struct {
	VXID string `json:"id"`
}

func (c *Client) CreatePlaceholder(ingestType placeholderType, title, email string) (string, error) {

	tpl := xmlRawMaterialPlaceholderTmpl
	switch ingestType {
	case PLACEHOLDER_TYPE_MASTER:
		tpl = xmlMasterPlaceholderTmpl
	case PLACEHOLDER_TYPE_RAWMATERIAL:
		tpl = xmlRawMaterialPlaceholderTmpl
	}

	var body bytes.Buffer
	tpl.Execute(&body, PlacholderTplData{
		Title: title,
		Email: email,
	})

	result, err := c.restyClient.R().
		SetHeader("content-type", "application/xml").
		SetBody(body.String()).
		SetResult(&IDOnlyResult{}).
		// Copied from NodeRed. I have no clue what VX-76 is.
		Post("/import/placeholder?container=1&settings=VX-76")

	if err != nil {
		return "", err
	}

	return result.Result().(*IDOnlyResult).VXID, nil
}

type fileState string

const (
	FILE_STATE_CLOSED fileState = "CLOSED"
	FILE_STATE_OPEN   fileState = "OPEN"
)

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

func (c *Client) GetAbsoluteStoragePath(storageID string) (string, error) {
	result, err := c.restyClient.R().
		SetResult(&StorageResult{}).
		Get("/storage/" + storageID + "/method")

	if err != nil {
		return "", err
	}
	for _, m := range result.Result().(*StorageResult).Methods {
		if strings.HasPrefix(m.URI, "file://") {
			return strings.TrimPrefix(m.URI, "file://"), nil
		}
	}

	return "", fmt.Errorf("No local storage found for storage ID %s", storageID)
}

func (c *Client) RegisterFile(filePath string, fileState fileState) (string, error) {

	// We need the absolute path to the storage in order to remove it from the file path
	// Yeah, briliant design. I know.
	storagePath, err := c.GetAbsoluteStoragePath(DEFAULT_STORAGE_ID)
	if err != nil {
		return "", err
	}

	// Remove the storage path from the file path
	filePath = strings.TrimPrefix(filePath, storagePath)

	// Do it in a slightly more complicated way, but make sure everything is encoded properly.
	requestURL, _ := url.Parse(c.baseURL)
	requestURL.Path += "/storage/" + url.PathEscape(DEFAULT_STORAGE_ID) + "/file"
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

func (c *Client) AddFileToPlaceholder(itemID, fileID, tag string, fileState fileState) string {
	// TODO: Unfinished, not needed ritht now
	requestURL, _ := url.Parse(c.baseURL)
	requestURL.Path += "/import/placeholder/" + url.PathEscape(itemID) + "/container"
	q := requestURL.Query()
	q.Set("fileId", fileID)

	if tag != "" {
		q.Set("tag", tag)
	}

	if fileState == FILE_STATE_OPEN {
		q.Set("growing", "true")
		q.Set("jobmetadata", "portal_groups:StringArray%3dAdmin")
		q.Set("overrideFastStart", "true")
		q.Set("requireFastStart", "true")
		q.Set("fastStartLength", "7200")
		q.Set("settings", "VX-76")
	} else {
		q.Set("growing", "false")
	}

	requestURL.RawQuery = q.Encode()

	return requestURL.String()
}

func (c *Client) AddShapeToItem(tag, itemID, fileID string) (string, error) {
	requestURL, _ := url.Parse(c.baseURL)
	requestURL.Path += "/item/" + url.PathEscape(itemID) + "/shape"
	q := requestURL.Query()
	q.Set("storageId", DEFAULT_STORAGE_ID)
	q.Set("fileId", fileID)
	q.Set("tag", tag)
	requestURL.RawQuery = q.Encode()

	result, err := c.restyClient.R().
		Post(requestURL.String())

	//TODO: make sure to not return until the shape is actually imported
	if err != nil {
		return "", err
	}

	return result.String(), nil
}

func (c *Client) AddSidecarToItem(itemID, filePath, language string) (string, error) {
	requestURL, _ := url.Parse(c.baseURL)
	requestURL.Path += "/import/sidecar/" + url.PathEscape(itemID)
	q := requestURL.Query()
	q.Set("sidecar", "file://"+filePath)
	q.Set("jobmetadata", "subtitleLanguage="+language)
	requestURL.RawQuery = q.Encode()

	result, err := c.restyClient.R().
		Post(requestURL.String())

	if err != nil {
		return "", err
	}

	return result.String(), nil
}
