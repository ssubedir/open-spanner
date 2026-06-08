package meter

import "context"

type Repository interface {
	Save(ctx context.Context, meter Meter) (Meter, error)
	Find(ctx context.Context, query Query) ([]Meter, error)
	Count(ctx context.Context) (int, error)
	Delete(ctx context.Context, query Query) error
}
