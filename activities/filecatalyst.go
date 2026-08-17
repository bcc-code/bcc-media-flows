package activities

import (
	"context"

	"github.com/bcc-code/bcc-media-flows/filecatalyst"
)

func (ua UtilActivities) PokeFileCatalyst(ctx context.Context, _ any) (bool, error) {
	// This is intentionally a fire-and forget function
	err := filecatalyst.PokeFileCatalyst(ctx)
	return err == nil, nil
}
