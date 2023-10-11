package batonactvities

import "github.com/bcc-code/bccm-flows/services/baton"

func newClient() *baton.Client {
	return baton.NewClient("http://10.12.128.27:8080/Baton/api/")
}
