package cantemo

import (
	"context"
	"github.com/bcc-code/bcc-media-flows/services/cantemo"
	"os"
)

type AddRelationParams struct {
	Parent string
	Child  string
}

func getClient() *cantemo.Client {
	urlBase := os.Getenv("CANTEMO_URL")
	token := os.Getenv("CANTEMO_TOKEN")
	return cantemo.NewClient(urlBase, token)
}

func AddRelation(ctx context.Context, params AddRelationParams) (any, error) {
	return nil, getClient().AddRelation(params.Parent, params.Child)
}
