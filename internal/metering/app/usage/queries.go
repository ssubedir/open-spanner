package usage

import (
	"context"
	"time"

	domainusage "open-spanner/internal/metering/domain/usage"
)

type ListQuery struct {
	Subject    string
	MeterName  string
	From       time.Time
	To         time.Time
	BucketSize domainusage.BucketSize
}

func (s *service) List(ctx context.Context, input ListQuery) ([]ListItemResult, error) {
	query, err := domainusage.NewQuery(
		input.Subject,
		input.MeterName,
		input.From,
		input.To,
		input.BucketSize,
	)
	if err != nil {
		return nil, err
	}

	buckets, err := s.usageRepo.Query(ctx, query)
	if err != nil {
		return nil, err
	}

	results := make([]ListItemResult, 0, len(buckets))
	for _, bucket := range buckets {
		results = append(results, bucketResultFromDomain(bucket))
	}

	return results, nil
}
