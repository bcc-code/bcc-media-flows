package activities

import (
	"context"

	"github.com/bcc-code/bcc-media-flows/services/ftp"
)

type FtpPlayoutRenameParams struct {
	From string
	To   string
}

type FtpPlayoutRenameResult struct{}

func (ua UtilActivities) FtpPlayoutRename(_ context.Context, params FtpPlayoutRenameParams) (*FtpPlayoutRenameResult, error) {
	client, err := ftp.Playout()
	if err != nil {
		return nil, err
	}
	defer client.Close()

	err = client.Rename(params.From, params.To)
	if err != nil {
		return nil, err
	}

	return nil, nil
}
