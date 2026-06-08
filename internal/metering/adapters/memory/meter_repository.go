package memory

import (
	"context"
	"errors"

	"open-spanner/internal/metering/domain"
	domainmeter "open-spanner/internal/metering/domain/meter"
)

type MeterRepository struct {
	store *Store
}

func NewMeterRepository(store *Store) *MeterRepository {
	return &MeterRepository{store: store}
}

func (r *MeterRepository) Save(ctx context.Context, meter domainmeter.Meter) (domainmeter.Meter, error) {
	r.store.mu.Lock()
	defer r.store.mu.Unlock()

	if _, exists := r.store.metersByName[meter.Name()]; exists {
		return domainmeter.Meter{}, errors.Join(domain.ErrConflict, errors.New("meter already exists"))
	}

	r.store.metersByID[meter.ID()] = meter
	r.store.metersByName[meter.Name()] = meter
	return meter, nil
}

func (r *MeterRepository) Find(ctx context.Context, query domainmeter.Query) ([]domainmeter.Meter, error) {
	r.store.mu.RLock()
	defer r.store.mu.RUnlock()

	if query.ID != "" {
		meter, exists := r.store.metersByID[query.ID]
		if !exists {
			return []domainmeter.Meter{}, nil
		}
		return []domainmeter.Meter{meter}, nil
	}

	if query.Name != "" {
		meter, exists := r.store.metersByName[query.Name]
		if !exists {
			return []domainmeter.Meter{}, nil
		}
		return []domainmeter.Meter{meter}, nil
	}

	meters := make([]domainmeter.Meter, 0, len(r.store.metersByID))
	for _, meter := range r.store.metersByID {
		meters = append(meters, meter)
	}

	return meters, nil
}
