package ftp

import (
	"github.com/bcc-code/bcc-media-flows/environment"
)

func Playout() (*Client, error) {
	cfg := environment.Get()
	return NewClient(cfg.PlayoutFTP.Address(), cfg.PlayoutFTP.Username(), cfg.PlayoutFTP.Password())
}
