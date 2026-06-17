package usage

import (
	"context"
	"time"

	"github.com/ssubedir/open-spanner/internal/metering/app/page"
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

func timeZero() time.Time {
	return time.Time{}
}
