package cantemo

import (
	"context"

	"github.com/bcc-code/bcc-media-flows/services/cantemo"
)

type AddRelationParams struct {
	Parent string
	Child  string
}

type Activities struct {
	Client *cantemo.Client
}

// Cantemo is replaced at boot with a client built from the configuration.
var Cantemo = &Activities{}

func (a Activities) AddRelation(ctx context.Context, params AddRelationParams) (any, error) {
	return nil, a.Client.AddRelation(params.Parent, params.Child)
}
