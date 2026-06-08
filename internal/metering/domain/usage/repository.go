package usage

import "context"

type Repository interface {
	Save(ctx context.Context, event Event) (Event, error)
	SaveBulk(ctx context.Context, idempotencyKey string, events []Event) (BulkSaveResult, error)
	Query(ctx context.Context, query Query) ([]Bucket, error)
	FindEvents(ctx context.Context, query EventQuery) (EventPage, error)
	CountEvents(ctx context.Context) (int, error)
	FindMeterStats(ctx context.Context) ([]MeterStats, error)
	FindSubjectStats(ctx context.Context, query SubjectStatsQuery) ([]SubjectStats, error)
	CountPrunableEvents(ctx context.Context, query PruneQuery) (int, error)
	PruneEvents(ctx context.Context, query PruneQuery) (int, error)
	SavePruneRun(ctx context.Context, run PruneRun) (PruneRun, error)
	FindPruneRuns(ctx context.Context, query RunQuery) ([]PruneRun, error)
	CountPruneRuns(ctx context.Context) (int, error)
	SaveIngestionRun(ctx context.Context, run IngestionRun) (IngestionRun, error)
	FindIngestionRuns(ctx context.Context, query RunQuery) ([]IngestionRun, error)
}
