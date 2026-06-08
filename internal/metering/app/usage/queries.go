package usage

import (
	"context"
	"time"

	"github.com/ssubedir/open-spanner/internal/metering/app/page"
	"github.com/ssubedir/open-spanner/internal/metering/domain"
	domainmeter "github.com/ssubedir/open-spanner/internal/metering/domain/meter"
	domainusage "github.com/ssubedir/open-spanner/internal/metering/domain/usage"
)

type ListQuery struct {
	Subject    string
	MeterName  string
	From       time.Time
	To         time.Time
	BucketSize domainusage.BucketSize
	Metadata   map[string]string
	GroupBy    string
	Limit      int
	Filter     domainusage.Filter
}

type EventListQuery struct {
	Subject   string
	MeterName string
	From      time.Time
	To        time.Time
	Limit     int
	Cursor    string
	Filter    domainusage.Filter
}

type PruneRunListQuery struct {
	Limit  int
	Cursor string
}

type IngestionListQuery struct {
	Limit  int
	Cursor string
}

func (s *service) ListPruneRuns(ctx context.Context, input PruneRunListQuery) (PruneRunListResult, error) {
	cursor, err := page.Decode(input.Cursor)
	if err != nil {
		return PruneRunListResult{}, err
	}

	limit := domainusage.NormalizeLimit(input.Limit)
	runs, err := s.usageRepo.FindPruneRuns(ctx, domainusage.NewRunQuery(limit+1, cursor.Time, cursor.ID))
	if err != nil {
		return PruneRunListResult{}, err
	}

	nextCursor := ""
	if len(runs) > limit {
		last := runs[limit-1]
		nextCursor, err = page.Encode(page.Cursor{Time: last.CreatedAt(), ID: last.ID()})
		if err != nil {
			return PruneRunListResult{}, err
		}
		runs = runs[:limit]
	}

	results := make([]PruneResult, 0, len(runs))
	for _, run := range runs {
		results = append(results, pruneResultFromDomain(run))
	}

	return PruneRunListResult{Items: results, NextCursor: nextCursor}, nil
}

func (s *service) ListIngestions(ctx context.Context, input IngestionListQuery) (IngestionListResult, error) {
	cursor, err := page.Decode(input.Cursor)
	if err != nil {
		return IngestionListResult{}, err
	}

	limit := domainusage.NormalizeLimit(input.Limit)
	runs, err := s.usageRepo.FindIngestionRuns(ctx, domainusage.NewRunQuery(limit+1, cursor.Time, cursor.ID))
	if err != nil {
		return IngestionListResult{}, err
	}

	nextCursor := ""
	if len(runs) > limit {
		last := runs[limit-1]
		nextCursor, err = page.Encode(page.Cursor{Time: last.CreatedAt(), ID: last.ID()})
		if err != nil {
			return IngestionListResult{}, err
		}
		runs = runs[:limit]
	}

	results := make([]IngestionResult, 0, len(runs))
	for _, run := range runs {
		results = append(results, ingestionResultFromDomain(run))
	}

	return IngestionListResult{Items: results, NextCursor: nextCursor}, nil
}

func (s *service) List(ctx context.Context, input ListQuery) ([]ListItemResult, error) {
	meters, err := s.meterRepo.Find(ctx, domainmeter.Query{Name: input.MeterName})
	if err != nil {
		return nil, err
	}
	if len(meters) == 0 {
		return nil, domain.ErrNotFound
	}
	meter := meters[0]
	if input.GroupBy != "" {
		if _, exists := meter.MetadataSchema()[input.GroupBy]; !exists {
			return nil, domain.ErrInvalidInput
		}
	}

	query, err := domainusage.NewFilteredQuery(
		input.Subject,
		meter.Name(),
		input.From,
		input.To,
		input.BucketSize,
		meter.Aggregation(),
		input.Metadata,
		input.GroupBy,
		input.Limit,
		input.Filter,
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
		results = append(results, bucketResultFromDomain(bucket, string(meter.Aggregation()), meter.Unit()))
	}

	return results, nil
}

func (s *service) ListEvents(ctx context.Context, input EventListQuery) (EventListResult, error) {
	cursor, err := decodeEventCursor(input.Cursor)
	if err != nil {
		return EventListResult{}, err
	}

	query, err := domainusage.NewFilteredEventQuery(
		input.Subject,
		input.MeterName,
		input.From,
		input.To,
		input.Limit,
		cursor,
		input.Filter,
	)
	if err != nil {
		return EventListResult{}, err
	}

	page, err := s.usageRepo.FindEvents(ctx, query)
	if err != nil {
		return EventListResult{}, err
	}

	events := page.Events()
	results := make([]Result, 0, len(events))
	for _, event := range events {
		results = append(results, eventResultFromDomain(event))
	}

	nextCursor, err := encodeEventCursor(page.NextCursor())
	if err != nil {
		return EventListResult{}, err
	}

	return EventListResult{Items: results, NextCursor: nextCursor}, nil
}

func decodeEventCursor(value string) (domainusage.EventCursor, error) {
	cursor, err := page.Decode(value)
	if err != nil {
		return domainusage.EventCursor{}, err
	}
	if cursor.Time.IsZero() && cursor.ID == "" {
		return domainusage.EventCursor{}, nil
	}

	return domainusage.NewEventCursor(cursor.Time, cursor.ID)
}

func encodeEventCursor(cursor domainusage.EventCursor) (string, error) {
	if cursor.IsZero() {
		return "", nil
	}

	return page.Encode(page.Cursor{Time: cursor.EventTime(), ID: cursor.ID()})
}
