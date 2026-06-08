package subject

import (
	"context"
	"time"

	"github.com/ssubedir/open-spanner/internal/metering/app/page"
	domainusage "github.com/ssubedir/open-spanner/internal/metering/domain/usage"
)

type Service interface {
	List(ctx context.Context, query ListQuery) (ListResult, error)
	ListEvents(ctx context.Context, query EventListQuery) ([]EventResult, error)
}

type ListQuery struct {
	Limit  int
	Cursor string
}

type EventListQuery struct {
	Subject string
	Limit   int
}

type Result struct {
	Subject     string
	UsageEvents int
	Meters      int
	LastEventAt time.Time
}

type ListResult struct {
	Items      []Result
	NextCursor string
}

type EventResult struct {
	ID             string
	IdempotencyKey string
	Subject        string
	MeterName      string
	Quantity       float64
	EventTime      time.Time
	ReceivedAt     time.Time
	Metadata       map[string]any
}

type service struct {
	usageRepo domainusage.Repository
}

func NewService(usageRepo domainusage.Repository) Service {
	return &service{usageRepo: usageRepo}
}

func (s *service) List(ctx context.Context, query ListQuery) (ListResult, error) {
	cursor, err := page.Decode(query.Cursor)
	if err != nil {
		return ListResult{}, err
	}

	limit := domainusage.NormalizeLimit(query.Limit)
	stats, err := s.usageRepo.FindSubjectStats(ctx, domainusage.NewSubjectStatsQuery(limit+1, cursor.Time, cursor.ID))
	if err != nil {
		return ListResult{}, err
	}

	nextCursor := ""
	if len(stats) > limit {
		last := stats[limit-1]
		nextCursor, err = page.Encode(page.Cursor{Time: last.LastEventAt(), ID: last.Subject()})
		if err != nil {
			return ListResult{}, err
		}
		stats = stats[:limit]
	}

	results := make([]Result, 0, len(stats))
	for _, stat := range stats {
		results = append(results, Result{
			Subject:     stat.Subject(),
			UsageEvents: stat.UsageEvents(),
			Meters:      stat.Meters(),
			LastEventAt: stat.LastEventAt(),
		})
	}

	return ListResult{Items: results, NextCursor: nextCursor}, nil
}

func (s *service) ListEvents(ctx context.Context, query EventListQuery) ([]EventResult, error) {
	eventQuery, err := domainusage.NewEventQuery(query.Subject, "", time.Time{}, time.Time{}, query.Limit, domainusage.EventCursor{})
	if err != nil {
		return nil, err
	}

	page, err := s.usageRepo.FindEvents(ctx, eventQuery)
	if err != nil {
		return nil, err
	}

	events := page.Events()
	results := make([]EventResult, 0, len(events))
	for _, event := range events {
		results = append(results, EventResult{
			ID:             event.ID(),
			IdempotencyKey: event.IdempotencyKey(),
			Subject:        event.Subject(),
			MeterName:      event.MeterName(),
			Quantity:       event.Quantity(),
			EventTime:      event.EventTime(),
			ReceivedAt:     event.ReceivedAt(),
			Metadata:       event.Metadata(),
		})
	}

	return results, nil
}
