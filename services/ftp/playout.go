package ftp

import (
	"os"
)

var (
	playoutIP       = os.Getenv("PLAYOUT_FTP_IP")
	playoutUser     = os.Getenv("PLAYOUT_FTP_USER")
	playoutPassword = os.Getenv("PLAYOUT_FTP_PASSWORD")
)

func Playout() (*Client, error) {
	return NewClient(playoutIP, playoutUser, playoutPassword)
}
