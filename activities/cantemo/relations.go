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
	return cantemo.NewFromConfig(environment.Get().Cantemo)
}

func AddRelation(ctx context.Context, params AddRelationParams) (any, error) {
	return nil, GetClient().AddRelation(params.Parent, params.Child)
}
