package usage

import "context"
import "time"

type Repository interface {
	Save(ctx context.Context, event Event) (Event, error)
	SaveBulk(ctx context.Context, idempotencyKey string, events []Event) (BulkSaveResult, error)
	Query(ctx context.Context, query Query) ([]Bucket, error)
	FindDimensionValues(ctx context.Context, query DimensionValueQuery) ([]DimensionValue, error)
	FindBreakdown(ctx context.Context, query BreakdownQuery) ([]BreakdownItem, error)
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
	SaveExportJob(ctx context.Context, job ExportJob) (ExportJob, error)
	FindExportJob(ctx context.Context, id string) (ExportJob, error)
	FindExportJobs(ctx context.Context, query RunQuery) ([]ExportJob, error)
	ClaimExportJob(ctx context.Context, now time.Time, lockedUntil time.Time, maxAttempts int) (ExportJob, error)
	CompleteExportJob(ctx context.Context, id string, artifactPath string, artifactSize int64, completedAt time.Time) (ExportJob, error)
	FailExportJob(ctx context.Context, id string, errorMessage string, failedAt time.Time) (ExportJob, error)
	CancelExportJob(ctx context.Context, id string, canceledAt time.Time) (ExportJob, error)
	RetryExportJob(ctx context.Context, id string, retriedAt time.Time) (ExportJob, error)
}
