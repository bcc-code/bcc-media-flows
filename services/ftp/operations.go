package ftp

func (c *Client) Rename(from, to string) error {
	return c.conn.Rename(from, to)
}
