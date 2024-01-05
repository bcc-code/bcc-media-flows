package batonactivities

import "github.com/bcc-code/bcc-media-flows/services/baton"

func getClient() *baton.Client {
	return baton.NewClient("http://10.12.128.27:8080/Baton/api/")
}
