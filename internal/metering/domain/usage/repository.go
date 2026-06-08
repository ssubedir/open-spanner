package usage

import "context"

type Repository interface {
	Save(ctx context.Context, event Event) (Event, error)
	Query(ctx context.Context, query Query) ([]Bucket, error)
}
