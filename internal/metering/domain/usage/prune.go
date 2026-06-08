package usage

import (
	"fmt"
	"strings"
	"time"

	"github.com/ssubedir/open-spanner/internal/metering/domain"
)

type PruneRun struct {
	id        string
	dryRun    bool
	deleted   int
	meters    []PruneRunMeter
	createdAt time.Time
}

type PruneRunMeter struct {
	meterName string
	before    time.Time
	deleted   int
}

func NewPruneRun(id string, dryRun bool, deleted int, meters []PruneRunMeter, createdAt time.Time) (PruneRun, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return PruneRun{}, fmt.Errorf("%w: prune run id is required", domain.ErrInvalidInput)
	}
	if deleted < 0 {
		return PruneRun{}, fmt.Errorf("%w: deleted count cannot be negative", domain.ErrInvalidInput)
	}
	if createdAt.IsZero() {
		return PruneRun{}, fmt.Errorf("%w: created at is required", domain.ErrInvalidInput)
	}

	metersCopy := make([]PruneRunMeter, len(meters))
	copy(metersCopy, meters)

	return PruneRun{
		id:        id,
		dryRun:    dryRun,
		deleted:   deleted,
		meters:    metersCopy,
		createdAt: createdAt.UTC(),
	}, nil
}

func NewPruneRunMeter(meterName string, before time.Time, deleted int) (PruneRunMeter, error) {
	meterName = strings.TrimSpace(meterName)
	if meterName == "" {
		return PruneRunMeter{}, fmt.Errorf("%w: meter is required", domain.ErrInvalidInput)
	}
	if before.IsZero() {
		return PruneRunMeter{}, fmt.Errorf("%w: prune cutoff is required", domain.ErrInvalidInput)
	}
	if deleted < 0 {
		return PruneRunMeter{}, fmt.Errorf("%w: deleted count cannot be negative", domain.ErrInvalidInput)
	}

	return PruneRunMeter{meterName: meterName, before: before.UTC(), deleted: deleted}, nil
}

func (r PruneRun) ID() string {
	return r.id
}

func (r PruneRun) DryRun() bool {
	return r.dryRun
}

func (r PruneRun) Deleted() int {
	return r.deleted
}

func (r PruneRun) Meters() []PruneRunMeter {
	meters := make([]PruneRunMeter, len(r.meters))
	copy(meters, r.meters)
	return meters
}

func (r PruneRun) CreatedAt() time.Time {
	return r.createdAt
}

func (m PruneRunMeter) MeterName() string {
	return m.meterName
}

func (m PruneRunMeter) Before() time.Time {
	return m.before
}

func (m PruneRunMeter) Deleted() int {
	return m.deleted
}
