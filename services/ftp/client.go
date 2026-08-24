package ftp

import (
	"errors"
	"time"

	"github.com/jlaffaye/ftp"
)

type Client struct {
	conn *ftp.ServerConn
}

func NewClient(addr, username, password string) (*Client, error) {
	conn, err := ftp.Dial(addr, ftp.DialWithTimeout(time.Second*5))
	if err != nil {
		return nil, err
	}

	err = conn.Login(username, password)
	if err != nil {
		return nil, errors.Join(err, conn.Quit())
	}

	return &Client{
		conn: conn,
	}, nil
}

func (c *Client) Close() error {
	return c.conn.Quit()
}
