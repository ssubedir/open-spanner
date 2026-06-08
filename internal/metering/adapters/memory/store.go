package memory

import (
	"sync"

	domainmeter "github.com/ssubedir/open-spanner/internal/metering/domain/meter"
	domainusage "github.com/ssubedir/open-spanner/internal/metering/domain/usage"
)

type Store struct {
	mu              sync.RWMutex
	metersByID      map[string]domainmeter.Meter
	metersByName    map[string]domainmeter.Meter
	events          []domainusage.Event
	pruneRuns       []domainusage.PruneRun
	ingestionRuns   []domainusage.IngestionRun
	idempotencyKeys map[string]domainusage.Event
	bulkKeys        map[string]domainusage.BulkSaveResult
}

func NewStore() *Store {
	return &Store{
		metersByID:      map[string]domainmeter.Meter{},
		metersByName:    map[string]domainmeter.Meter{},
		idempotencyKeys: map[string]domainusage.Event{},
		bulkKeys:        map[string]domainusage.BulkSaveResult{},
	}
}
