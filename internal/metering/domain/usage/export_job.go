package usage

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/ssubedir/open-spanner/internal/metering/domain"
)

type ExportJobKind string

const (
	ExportJobUsageBuckets ExportJobKind = "usage_buckets"
)

type ExportJobStatus string

const (
	ExportJobQueued    ExportJobStatus = "queued"
	ExportJobRunning   ExportJobStatus = "running"
	ExportJobCompleted ExportJobStatus = "completed"
	ExportJobFailed    ExportJobStatus = "failed"
)

type ExportJobFormat string

const (
	ExportJobCSV ExportJobFormat = "csv"
)

type ExportJob struct {
	id           string
	kind         ExportJobKind
	status       ExportJobStatus
	format       ExportJobFormat
	queryJSON    string
	errorMessage string
	createdAt    time.Time
	updatedAt    time.Time
	completedAt  time.Time
}

func NewExportJob(id string, kind ExportJobKind, status ExportJobStatus, format ExportJobFormat, queryJSON string, errorMessage string, createdAt time.Time, updatedAt time.Time, completedAt time.Time) (ExportJob, error) {
	id = strings.TrimSpace(id)
	queryJSON = strings.TrimSpace(queryJSON)
	errorMessage = strings.TrimSpace(errorMessage)

	if id == "" {
		return ExportJob{}, fmt.Errorf("%w: export job id is required", domain.ErrInvalidInput)
	}
	if kind != ExportJobUsageBuckets {
		return ExportJob{}, fmt.Errorf("%w: export job kind is invalid", domain.ErrInvalidInput)
	}
	switch status {
	case ExportJobQueued, ExportJobRunning, ExportJobCompleted, ExportJobFailed:
	default:
		return ExportJob{}, fmt.Errorf("%w: export job status is invalid", domain.ErrInvalidInput)
	}
	if format != ExportJobCSV {
		return ExportJob{}, fmt.Errorf("%w: export job format is invalid", domain.ErrInvalidInput)
	}
	if queryJSON == "" || !json.Valid([]byte(queryJSON)) {
		return ExportJob{}, fmt.Errorf("%w: export job query must be valid JSON", domain.ErrInvalidInput)
	}
	if createdAt.IsZero() {
		return ExportJob{}, fmt.Errorf("%w: export job created at is required", domain.ErrInvalidInput)
	}
	if updatedAt.IsZero() {
		return ExportJob{}, fmt.Errorf("%w: export job updated at is required", domain.ErrInvalidInput)
	}
	if completedAt.IsZero() && (status == ExportJobCompleted || status == ExportJobFailed) {
		return ExportJob{}, fmt.Errorf("%w: export job completed at is required", domain.ErrInvalidInput)
	}

	return ExportJob{
		id:           id,
		kind:         kind,
		status:       status,
		format:       format,
		queryJSON:    queryJSON,
		errorMessage: errorMessage,
		createdAt:    createdAt.UTC(),
		updatedAt:    updatedAt.UTC(),
		completedAt:  completedAt.UTC(),
	}, nil
}

func (j ExportJob) ID() string {
	return j.id
}

func (j ExportJob) Kind() ExportJobKind {
	return j.kind
}

func (j ExportJob) Status() ExportJobStatus {
	return j.status
}

func (j ExportJob) Format() ExportJobFormat {
	return j.format
}

func (j ExportJob) QueryJSON() string {
	return j.queryJSON
}

func (j ExportJob) ErrorMessage() string {
	return j.errorMessage
}

func (j ExportJob) CreatedAt() time.Time {
	return j.createdAt
}

func (j ExportJob) UpdatedAt() time.Time {
	return j.updatedAt
}

func (j ExportJob) CompletedAt() time.Time {
	return j.completedAt
}
