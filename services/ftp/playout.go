package ftp

import (
	"os"
)

var (
	playoutIP       = os.Getenv("PLAYOUT_FTP_ADDRESS")
	playoutUser     = os.Getenv("PLAYOUT_FTP_USERNAME")
	playoutPassword = os.Getenv("PLAYOUT_FTP_PASSWORD")
)

func Playout() (*Client, error) {
	return NewClient(playoutIP, playoutUser, playoutPassword)
}
