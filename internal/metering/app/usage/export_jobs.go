package usage

import (
	"context"
	"errors"
	"time"

	"github.com/ssubedir/open-spanner/internal/metering/app/page"
	"github.com/ssubedir/open-spanner/internal/metering/domain"
	domainusage "github.com/ssubedir/open-spanner/internal/metering/domain/usage"
)

type ExportJobCreateCommand struct {
	Kind      string
	Format    string
	QueryJSON string
}

type ExportJobListQuery struct {
	Limit  int
	Cursor string
}

type ExportJobClaimCommand struct {
	LockTTL     time.Duration
	MaxAttempts int
}

type ExportJobCompleteCommand struct {
	ID           string
	ArtifactPath string
	ArtifactSize int64
}

type ExportJobFailCommand struct {
	ID           string
	ErrorMessage string
}

type ExportJobCancelCommand struct {
	ID string
}

type ExportJobRetryCommand struct {
	ID string
}

func (s *service) CreateExportJob(ctx context.Context, cmd ExportJobCreateCommand) (ExportJobResult, error) {
	kind := domainusage.ExportJobKind(cmd.Kind)
	if kind == "" {
		kind = domainusage.ExportJobUsageBuckets
	}
	format := domainusage.ExportJobFormat(cmd.Format)
	if format == "" {
		format = domainusage.ExportJobCSV
	}

	now := s.now()
	job, err := domainusage.NewExportJob(
		newID(),
		kind,
		domainusage.ExportJobQueued,
		format,
		cmd.QueryJSON,
		"",
		0,
		timeZero(),
		"",
		0,
		now,
		now,
		timeZero(),
	)
	if err != nil {
		return ExportJobResult{}, err
	}

	job, err = s.usageRepo.SaveExportJob(ctx, job)
	if err != nil {
		return ExportJobResult{}, err
	}

	return exportJobResultFromDomain(job), nil
}

func (s *service) GetExportJob(ctx context.Context, id string) (ExportJobResult, error) {
	job, err := s.usageRepo.FindExportJob(ctx, id)
	if err != nil {
		return ExportJobResult{}, err
	}
	return exportJobResultFromDomain(job), nil
}

func (s *service) ListExportJobs(ctx context.Context, input ExportJobListQuery) (ExportJobListResult, error) {
	cursor, err := page.Decode(input.Cursor)
	if err != nil {
		return ExportJobListResult{}, err
	}

	limit := domainusage.NormalizeLimit(input.Limit)
	jobs, err := s.usageRepo.FindExportJobs(ctx, domainusage.NewRunQuery(limit+1, cursor.Time, cursor.ID))
	if err != nil {
		return ExportJobListResult{}, err
	}

	nextCursor := ""
	if len(jobs) > limit {
		last := jobs[limit-1]
		nextCursor, err = page.Encode(page.Cursor{Time: last.CreatedAt(), ID: last.ID()})
		if err != nil {
			return ExportJobListResult{}, err
		}
		jobs = jobs[:limit]
	}

	results := make([]ExportJobResult, 0, len(jobs))
	for _, job := range jobs {
		results = append(results, exportJobResultFromDomain(job))
	}

	return ExportJobListResult{Items: results, NextCursor: nextCursor}, nil
}

func (s *service) ClaimExportJob(ctx context.Context, cmd ExportJobClaimCommand) (ExportJobResult, bool, error) {
	if cmd.LockTTL <= 0 {
		return ExportJobResult{}, false, domain.ErrInvalidInput
	}
	if cmd.MaxAttempts <= 0 {
		return ExportJobResult{}, false, domain.ErrInvalidInput
	}

	now := s.now()
	job, err := s.usageRepo.ClaimExportJob(ctx, now, now.Add(cmd.LockTTL), cmd.MaxAttempts)
	if errors.Is(err, domain.ErrNotFound) {
		return ExportJobResult{}, false, nil
	}
	if err != nil {
		return ExportJobResult{}, false, err
	}

	return exportJobResultFromDomain(job), true, nil
}

func (s *service) CompleteExportJob(ctx context.Context, cmd ExportJobCompleteCommand) (ExportJobResult, error) {
	job, err := s.usageRepo.CompleteExportJob(ctx, cmd.ID, cmd.ArtifactPath, cmd.ArtifactSize, s.now())
	if err != nil {
		return ExportJobResult{}, err
	}
	return exportJobResultFromDomain(job), nil
}

func (s *service) FailExportJob(ctx context.Context, cmd ExportJobFailCommand) (ExportJobResult, error) {
	job, err := s.usageRepo.FailExportJob(ctx, cmd.ID, cmd.ErrorMessage, s.now())
	if err != nil {
		return ExportJobResult{}, err
	}
	return exportJobResultFromDomain(job), nil
}

func (s *service) CancelExportJob(ctx context.Context, cmd ExportJobCancelCommand) (ExportJobResult, error) {
	current, err := s.usageRepo.FindExportJob(ctx, cmd.ID)
	if err != nil {
		return ExportJobResult{}, err
	}
	if current.Status() != domainusage.ExportJobQueued && current.Status() != domainusage.ExportJobRunning {
		return ExportJobResult{}, domain.ErrConflict
	}

	job, err := s.usageRepo.CancelExportJob(ctx, cmd.ID, s.now())
	if err != nil {
		return ExportJobResult{}, err
	}
	return exportJobResultFromDomain(job), nil
}

func (s *service) RetryExportJob(ctx context.Context, cmd ExportJobRetryCommand) (ExportJobResult, error) {
	current, err := s.usageRepo.FindExportJob(ctx, cmd.ID)
	if err != nil {
		return ExportJobResult{}, err
	}
	if current.Status() != domainusage.ExportJobFailed && current.Status() != domainusage.ExportJobCanceled {
		return ExportJobResult{}, domain.ErrConflict
	}

	job, err := s.usageRepo.RetryExportJob(ctx, cmd.ID, s.now())
	if err != nil {
		return ExportJobResult{}, err
	}
	return exportJobResultFromDomain(job), nil
}

func timeZero() time.Time {
	return time.Time{}
}
