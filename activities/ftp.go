package activities

import (
	"context"
	"github.com/bcc-code/bccm-flows/services/ftp"
)

type FtpPlayoutRenameParams struct {
	From string
	To   string
}

func FtpPlayoutRename(ctx context.Context, params FtpPlayoutRenameParams) error {
	client, err := ftp.Playout()
	if err != nil {
		return err
	}
	defer client.Close()

	err = client.Rename(params.From, params.To)
	if err != nil {
		return err
	}

	return nil
}
