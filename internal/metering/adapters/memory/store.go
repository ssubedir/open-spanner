package memory

import (
	"sync"

	domainmeter "open-spanner/internal/metering/domain/meter"
	domainusage "open-spanner/internal/metering/domain/usage"
)

type Store struct {
	mu              sync.RWMutex
	metersByID      map[string]domainmeter.Meter
	metersByName    map[string]domainmeter.Meter
	events          []domainusage.Event
	idempotencyKeys map[string]domainusage.Event
}

func NewStore() *Store {
	return &Store{
		metersByID:      map[string]domainmeter.Meter{},
		metersByName:    map[string]domainmeter.Meter{},
		idempotencyKeys: map[string]domainusage.Event{},
	}
}
