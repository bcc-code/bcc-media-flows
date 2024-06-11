package cantemo

import (
	"context"
	"github.com/bcc-code/bcc-media-flows/services/cantemo"
	"strings"
)

type GetFilesParams struct {
	Path     string
	State    string
	Storages []string
	Page     int
	Query    string
}

func GetFiles(_ context.Context, params GetFilesParams) (*cantemo.GetFilesResult, error) {
	return GetClient().GetFiles(
		params.Path,
		params.State,
		strings.Join(params.Storages, ","),
		params.Page,
		params.Query,
	)
}

type RenameFileParams struct {
	ItemID    string
	ShapeID   string
	StorageID string
	NewPath   string
}

func RenameFile(_ context.Context, params *RenameFileParams) (any, error) {
	return nil, GetClient().RenameFile(params.ItemID, params.ShapeID, params.StorageID, params.NewPath)
}
