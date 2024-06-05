package cantemo

import (
	"context"
	"os"

	"github.com/bcc-code/bcc-media-flows/services/cantemo"
)

type AddRelationParams struct {
	Parent string
	Child  string
}

func GetClient() *cantemo.Client {
	urlBase := os.Getenv("CANTEMO_URL")
	token := os.Getenv("CANTEMO_TOKEN")
	return cantemo.NewClient(urlBase, token)
}

func AddRelation(ctx context.Context, params AddRelationParams) (any, error) {
	return nil, GetClient().AddRelation(params.Parent, params.Child)
}
