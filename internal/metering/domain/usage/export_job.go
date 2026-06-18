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
	ExportJobCanceled  ExportJobStatus = "canceled"
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
	attempts     int
	lockedUntil  time.Time
	artifactPath string
	artifactSize int64
	createdAt    time.Time
	updatedAt    time.Time
	completedAt  time.Time
}

func NewExportJob(id string, kind ExportJobKind, status ExportJobStatus, format ExportJobFormat, queryJSON string, errorMessage string, attempts int, lockedUntil time.Time, artifactPath string, artifactSize int64, createdAt time.Time, updatedAt time.Time, completedAt time.Time) (ExportJob, error) {
	id = strings.TrimSpace(id)
	queryJSON = strings.TrimSpace(queryJSON)
	errorMessage = strings.TrimSpace(errorMessage)
	artifactPath = strings.TrimSpace(artifactPath)

	if id == "" {
		return ExportJob{}, fmt.Errorf("%w: export job id is required", domain.ErrInvalidInput)
	}
	if kind != ExportJobUsageBuckets {
		return ExportJob{}, fmt.Errorf("%w: export job kind is invalid", domain.ErrInvalidInput)
	}
	switch status {
	case ExportJobQueued, ExportJobRunning, ExportJobCompleted, ExportJobFailed, ExportJobCanceled:
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
	if completedAt.IsZero() && (status == ExportJobCompleted || status == ExportJobFailed || status == ExportJobCanceled) {
		return ExportJob{}, fmt.Errorf("%w: export job completed at is required", domain.ErrInvalidInput)
	}
	if status == ExportJobRunning && lockedUntil.IsZero() {
		return ExportJob{}, fmt.Errorf("%w: export job lock is required", domain.ErrInvalidInput)
	}
	if status == ExportJobCompleted && artifactPath == "" {
		return ExportJob{}, fmt.Errorf("%w: export job artifact path is required", domain.ErrInvalidInput)
	}
	if attempts < 0 {
		return ExportJob{}, fmt.Errorf("%w: export job attempts cannot be negative", domain.ErrInvalidInput)
	}
	if artifactSize < 0 {
		return ExportJob{}, fmt.Errorf("%w: export job artifact size cannot be negative", domain.ErrInvalidInput)
	}

	return ExportJob{
		id:           id,
		kind:         kind,
		status:       status,
		format:       format,
		queryJSON:    queryJSON,
		errorMessage: errorMessage,
		attempts:     attempts,
		lockedUntil:  lockedUntil.UTC(),
		artifactPath: artifactPath,
		artifactSize: artifactSize,
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

func (j ExportJob) Attempts() int {
	return j.attempts
}

func (j ExportJob) LockedUntil() time.Time {
	return j.lockedUntil
}

func (j ExportJob) ArtifactPath() string {
	return j.artifactPath
}

func (j ExportJob) ArtifactSize() int64 {
	return j.artifactSize
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
