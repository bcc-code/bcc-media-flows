package ftp

import (
	"github.com/bcc-code/bcc-media-flows/environment"
)

func Playout() (*Client, error) {
	cfg := environment.Get()
	return NewClient(cfg.PlayoutFTPAddress, cfg.PlayoutFTPUsername, cfg.PlayoutFTPPassword)
}
