package vsapi

import (
	"bytes"
	"fmt"
	"net/url"

	"github.com/orsinium-labs/enum"
)

// PlaceholderType selects which metadata template a new placeholder is created
// from, and so which group the item lands in.
type PlaceholderType enum.Member[string]

var (
	PlaceholderTypeMaster = PlaceholderType{Value: "master"}
	PlaceholderTypeRaw    = PlaceholderType{Value: "raw"}
	PlaceholderTypes      = enum.New(PlaceholderTypeMaster, PlaceholderTypeRaw)
)

// FileState is Vidispine's own name for whether a file is still being written
// to. Registering a file as OPEN is what makes it a growing file.
type FileState enum.Member[string]

var (
	FileStateClosed = FileState{Value: "CLOSED"}
	FileStateOpen   = FileState{Value: "OPEN"}
	FileStates      = enum.New(FileStateClosed, FileStateOpen)
)

type PlacholderTplData struct {
	Title string
}

func (c *Client) CreatePlaceholder(ingestType PlaceholderType, title string) (string, error) {

	tpl := xmlRawMaterialPlaceholderTmpl
	switch ingestType {
	case PlaceholderTypeMaster:
		tpl = xmlMasterPlaceholderTmpl
	case PlaceholderTypeRaw:
		tpl = xmlRawMaterialPlaceholderTmpl
	}

	var body bytes.Buffer
	tpl.Execute(&body, PlacholderTplData{
		Title: title,
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

func (c *Client) AddFileToPlaceholder(itemID, fileID, tag string, fileState FileState) (string, error) {
	requestURL, err := url.Parse(c.baseURL)
	if err != nil {
		return "", err
	}
	requestURL.Path += "/import/placeholder/" + url.PathEscape(itemID) + "/container"
	q := requestURL.Query()
	q.Set("fileId", fileID)

	if tag != "" {
		q.Set("tag", tag)
	}

	if fileState == FileStateOpen {
		q.Set("growing", "true")
		q.Set("jobmetadata", "portal_groups:StringArray=Admin")
		//q.Set("overrideFastStart", "true")
		//q.Set("requireFastStart", "true")
		//q.Set("fastStartLength", "7200")
		q.Set("settings", "VX-76")
	} else {
		q.Set("growing", "false")
	}

	requestURL.RawQuery = q.Encode()

	result, err := c.restyClient.R().
		SetResult(&JobDocument{}).
		SetHeader("content-type", "application/json").
		Post(requestURL.String())

	if err != nil {
		return "", err
	}

	return result.Result().(*JobDocument).JobID, nil
}

func (c *Client) CreateThumbnails(itemID string, width, height int) (string, error) {
	result, err := c.restyClient.R().
		SetHeader("content-type", "application/xml").
		SetHeader("accept", "application/json").
		SetResult(&JobDocument{}).
		Post(fmt.Sprintf("/item/%s/thumbnail?createThumbnails=true&thumbnailWidth=%d&thumbnailHeight=%d", url.PathEscape(itemID), width, height))

	if err != nil {
		return "", err
	}

	return result.Result().(*JobDocument).JobID, err
}
