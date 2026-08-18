package cantemo

import (
	"context"
	"github.com/bcc-code/bcc-media-flows/environment"

	"github.com/bcc-code/bcc-media-flows/services/cantemo"
)

type AddRelationParams struct {
	Parent string
	Child  string
}

func GetClient() *cantemo.Client {
	urlBase := environment.Get().CantemoURL
	token := environment.Get().CantemoToken
	return cantemo.NewClient(urlBase, token)
}

func AddRelation(ctx context.Context, params AddRelationParams) (any, error) {
	return nil, GetClient().AddRelation(params.Parent, params.Child)
}
