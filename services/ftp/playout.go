package ftp

import (
	"os"
)

func Playout() (*Client, error) {
	return NewClient(
		os.Getenv("PLAYOUT_FTP_ADDRESS"),
		os.Getenv("PLAYOUT_FTP_USERNAME"),
		os.Getenv("PLAYOUT_FTP_PASSWORD"),
	)
}
