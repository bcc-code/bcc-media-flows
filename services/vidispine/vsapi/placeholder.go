package vsapi

import (
	"bytes"
	"net/url"
)

type PlaceholderType string

const (
	PlaceholderTypeMaster PlaceholderType = "master"
	PlaceholderTypeRaw    PlaceholderType = "raw"
)

type FileState string

const (
	FileStateClosed FileState = "CLOSED"
	FileStateOpen   FileState = "OPEN"
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

func (c *Client) AddFileToPlaceholder(itemID, fileID, tag string, fileState FileState) string {
	panic("Not implemented")
	// TODO: Unfinished, not needed ritht now
	requestURL, _ := url.Parse(c.baseURL)
	requestURL.Path += "/import/placeholder/" + url.PathEscape(itemID) + "/container"
	q := requestURL.Query()
	q.Set("fileId", fileID)

	if tag != "" {
		q.Set("tag", tag)
	}

	if fileState == FileStateOpen {
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
