package meter

import "context"

type Repository interface {
	Save(ctx context.Context, meter Meter) (Meter, error)
	Find(ctx context.Context, query Query) ([]Meter, error)
}
