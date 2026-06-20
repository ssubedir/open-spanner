package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	appauth "github.com/ssubedir/open-spanner/internal/auth"
	"github.com/ssubedir/open-spanner/internal/metering/adapters/sqlite/sqlitedb"
	"github.com/ssubedir/open-spanner/internal/metering/domain"
	domainmeter "github.com/ssubedir/open-spanner/internal/metering/domain/meter"
)

type MeterRepository struct {
	store   *Store
	queries *sqlitedb.Queries
}

func NewMeterRepository(store *Store) *MeterRepository {
	return &MeterRepository{store: store, queries: sqlitedb.New(store)}
}

func (r *MeterRepository) Save(ctx context.Context, meter domainmeter.Meter) (domainmeter.Meter, error) {
	workspaceID, err := appauth.RequireWorkspaceID(ctx)
	if err != nil {
		return domainmeter.Meter{}, err
	}
	dimensions, err := marshalDimensions(meter.Dimensions())
	if err != nil {
		return domainmeter.Meter{}, err
	}

	err = r.store.WithinTransaction(ctx, func(txCtx context.Context) error {
		existing, err := queriesFor(txCtx, r.queries).ListMeters(txCtx, sqlitedb.ListMetersParams{
			WorkspaceID: workspaceID,
			ID:          optionalValue(meter.ID()),
			Limit:       1,
		})
		if err != nil {
			return err
		}

		err = queriesFor(txCtx, r.queries).SaveMeter(txCtx, sqlitedb.SaveMeterParams{
			ID:                 meter.ID(),
			WorkspaceID:        workspaceID,
			Name:               meter.Name(),
			Description:        meter.Description(),
			Unit:               meter.Unit(),
			Aggregation:        string(meter.Aggregation()),
			Dimensions:         dimensions,
			EventRetentionDays: int64(meter.EventRetentionDays()),
			CreatedAt:          formatTime(meter.CreatedAt()),
		})
		if err != nil {
			if isUniqueConstraint(err) {
				return errors.Join(domain.ErrConflict, err)
			}
			return err
		}

		if len(existing) > 0 {
			return nil
		}

		return queriesFor(txCtx, r.queries).IncrementWorkspaceMeters(txCtx, sqlitedb.IncrementWorkspaceMetersParams{
			WorkspaceID: workspaceID,
			Delta:       1,
			UpdatedAt:   formatTime(time.Now().UTC()),
		})
	})
	if err != nil {
		return domainmeter.Meter{}, err
	}

	return meter, nil
}

func (r *MeterRepository) Find(ctx context.Context, query domainmeter.Query) ([]domainmeter.Meter, error) {
	workspaceID, err := appauth.RequireWorkspaceID(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := queriesFor(ctx, r.queries).ListMeters(ctx, sqlitedb.ListMetersParams{
		WorkspaceID: workspaceID,
		ID:          optionalValue(query.ID),
		Name:        optionalValue(query.Name),
		Cursor:      optionalValue(query.Cursor),
		Limit:       int64(domainmeter.NormalizeLimit(query.Limit)),
	})
	if err != nil {
		return nil, err
	}

	meters := make([]domainmeter.Meter, 0, len(rows))
	for _, row := range rows {
		meter, err := meterFromFields(
			row.ID,
			row.Name,
			row.Description,
			row.Unit,
			row.Aggregation,
			row.Dimensions,
			int(row.EventRetentionDays),
			row.CreatedAt,
		)
		if err != nil {
			return nil, err
		}
		meters = append(meters, meter)
	}

	return meters, nil
}

func (r *MeterRepository) Count(ctx context.Context) (int, error) {
	workspaceID, err := appauth.RequireWorkspaceID(ctx)
	if err != nil {
		return 0, err
	}
	count, err := queriesFor(ctx, r.queries).CountMeters(ctx, workspaceID)
	if err != nil {
		return 0, err
	}
	return int(count), nil
}

func (r *MeterRepository) Delete(ctx context.Context, query domainmeter.Query) error {
	workspaceID, err := appauth.RequireWorkspaceID(ctx)
	if err != nil {
		return err
	}
	meters, err := r.Find(ctx, query)
	if err != nil {
		return err
	}
	if len(meters) == 0 {
		return domain.ErrNotFound
	}

	meter := meters[0]
	usageCount, err := queriesFor(ctx, r.queries).CountUsageEventsForMeter(ctx, sqlitedb.CountUsageEventsForMeterParams{
		WorkspaceID: workspaceID,
		MeterName:   meter.Name(),
	})
	if err != nil {
		return err
	}
	if usageCount > 0 {
		return errors.Join(domain.ErrConflict, errors.New("meter has usage"))
	}

	return r.store.WithinTransaction(ctx, func(txCtx context.Context) error {
		if err := queriesFor(txCtx, r.queries).DeleteMeter(txCtx, sqlitedb.DeleteMeterParams{
			WorkspaceID: workspaceID,
			ID:          meter.ID(),
		}); err != nil {
			return err
		}

		return queriesFor(txCtx, r.queries).IncrementWorkspaceMeters(txCtx, sqlitedb.IncrementWorkspaceMetersParams{
			WorkspaceID: workspaceID,
			Delta:       -1,
			UpdatedAt:   formatTime(time.Now().UTC()),
		})
	})
}

func scanMeter(scanner interface {
	Scan(dest ...any) error
}) (domainmeter.Meter, error) {
	var id string
	var name string
	var description string
	var unit string
	var aggregation string
	var dimensionsText string
	var eventRetentionDays int
	var createdAtText string

	if err := scanner.Scan(&id, &name, &description, &unit, &aggregation, &dimensionsText, &eventRetentionDays, &createdAtText); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domainmeter.Meter{}, domain.ErrNotFound
		}
		return domainmeter.Meter{}, err
	}

	return meterFromFields(id, name, description, unit, aggregation, dimensionsText, eventRetentionDays, createdAtText)
}

func meterFromFields(id string, name string, description string, unit string, aggregation string, dimensionsText string, eventRetentionDays int, createdAtText string) (domainmeter.Meter, error) {
	createdAt, err := time.Parse(time.RFC3339Nano, createdAtText)
	if err != nil {
		return domainmeter.Meter{}, err
	}
	dimensions, err := unmarshalDimensions(dimensionsText)
	if err != nil {
		return domainmeter.Meter{}, err
	}

	return domainmeter.NewWithDimensions(id, name, description, unit, domainmeter.Aggregation(aggregation), dimensions, eventRetentionDays, createdAt)
}

func optionalValue(value string) any {
	if value == "" {
		return nil
	}
	return value
}

type dimensionPayload struct {
	Name        string `json:"name"`
	DisplayName string `json:"display_name,omitempty"`
	Description string `json:"description,omitempty"`
	Type        string `json:"type"`
	Required    bool   `json:"required"`
	Deprecated  bool   `json:"deprecated,omitempty"`
}

func marshalDimensions(dimensions []domainmeter.Dimension) (string, error) {
	payload := make([]dimensionPayload, 0, len(dimensions))
	for _, dimension := range dimensions {
		payload = append(payload, dimensionPayload{
			Name:        dimension.Name(),
			DisplayName: dimension.DisplayName(),
			Description: dimension.Description(),
			Type:        string(dimension.Type()),
			Required:    dimension.Required(),
			Deprecated:  dimension.Deprecated(),
		})
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func unmarshalDimensions(payload string) ([]domainmeter.Dimension, error) {
	if payload == "" {
		payload = "[]"
	}
	values := []dimensionPayload{}
	if err := json.Unmarshal([]byte(payload), &values); err != nil {
		return nil, err
	}

	dimensions := make([]domainmeter.Dimension, 0, len(values))
	for _, value := range values {
		dimension, err := domainmeter.NewDimension(value.Name, domainmeter.MetadataType(value.Type), value.DisplayName, value.Description, value.Required, value.Deprecated)
		if err != nil {
			return nil, err
		}
		dimensions = append(dimensions, dimension)
	}
	return dimensions, nil
}
