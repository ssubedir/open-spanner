package usage

import (
	"fmt"
	"strings"
	"time"

	"github.com/ssubedir/open-spanner/internal/metering/domain"
)

type IngestionKind string

const (
	IngestionSingle IngestionKind = "single"
	IngestionBulk   IngestionKind = "bulk"
	IngestionStream IngestionKind = "stream"
)

type IngestionRun struct {
	id         string
	kind       IngestionKind
	accepted   int
	duplicates int
	failed     int
	createdAt  time.Time
}

func NewIngestionRun(id string, kind IngestionKind, accepted int, duplicates int, failed int, createdAt time.Time) (IngestionRun, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return IngestionRun{}, fmt.Errorf("%w: ingestion id is required", domain.ErrInvalidInput)
	}
	if kind != IngestionSingle && kind != IngestionBulk && kind != IngestionStream {
		return IngestionRun{}, fmt.Errorf("%w: ingestion kind is invalid", domain.ErrInvalidInput)
	}
	if accepted < 0 || duplicates < 0 || failed < 0 {
		return IngestionRun{}, fmt.Errorf("%w: ingestion counts cannot be negative", domain.ErrInvalidInput)
	}
	if createdAt.IsZero() {
		return IngestionRun{}, fmt.Errorf("%w: ingestion created at is required", domain.ErrInvalidInput)
	}

	return IngestionRun{
		id:         id,
		kind:       kind,
		accepted:   accepted,
		duplicates: duplicates,
		failed:     failed,
		createdAt:  createdAt.UTC(),
	}, nil
}

func (r IngestionRun) ID() string {
	return r.id
}

func (r IngestionRun) Kind() IngestionKind {
	return r.kind
}

func (r IngestionRun) Accepted() int {
	return r.accepted
}

func (r IngestionRun) Duplicates() int {
	return r.duplicates
}

func (r IngestionRun) Failed() int {
	return r.failed
}

func (r IngestionRun) CreatedAt() time.Time {
	return r.createdAt
}
