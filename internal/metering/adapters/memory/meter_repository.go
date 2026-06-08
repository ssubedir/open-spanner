package memory

import (
	"context"
	"errors"
	"sort"

	"github.com/ssubedir/open-spanner/internal/metering/domain"
	domainmeter "github.com/ssubedir/open-spanner/internal/metering/domain/meter"
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

	if existing, exists := r.store.metersByName[meter.Name()]; exists && existing.ID() != meter.ID() {
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
	sort.Slice(meters, func(i, j int) bool {
		return meters[i].Name() < meters[j].Name()
	})

	if query.Cursor != "" {
		paged := make([]domainmeter.Meter, 0, len(meters))
		for _, meter := range meters {
			if meter.Name() > query.Cursor {
				paged = append(paged, meter)
			}
		}
		meters = paged
	}

	return limitMeters(meters, query.Limit), nil
}

func (r *MeterRepository) Count(ctx context.Context) (int, error) {
	r.store.mu.RLock()
	defer r.store.mu.RUnlock()

	return len(r.store.metersByID), nil
}

func (r *MeterRepository) Delete(ctx context.Context, query domainmeter.Query) error {
	r.store.mu.Lock()
	defer r.store.mu.Unlock()

	meters := r.findLocked(query)
	if len(meters) == 0 {
		return domain.ErrNotFound
	}

	meter := meters[0]
	for _, event := range r.store.events {
		if event.MeterName() == meter.Name() {
			return errors.Join(domain.ErrConflict, errors.New("meter has usage"))
		}
	}

	delete(r.store.metersByID, meter.ID())
	delete(r.store.metersByName, meter.Name())
	return nil
}

func (r *MeterRepository) findLocked(query domainmeter.Query) []domainmeter.Meter {
	if query.ID != "" {
		meter, exists := r.store.metersByID[query.ID]
		if !exists {
			return []domainmeter.Meter{}
		}
		return []domainmeter.Meter{meter}
	}

	if query.Name != "" {
		meter, exists := r.store.metersByName[query.Name]
		if !exists {
			return []domainmeter.Meter{}
		}
		return []domainmeter.Meter{meter}
	}

	meters := make([]domainmeter.Meter, 0, len(r.store.metersByID))
	for _, meter := range r.store.metersByID {
		meters = append(meters, meter)
	}

	return meters
}

func limitMeters(meters []domainmeter.Meter, limit int) []domainmeter.Meter {
	limit = domainmeter.NormalizeLimit(limit)
	if limit < len(meters) {
		return meters[:limit]
	}
	return meters
}
