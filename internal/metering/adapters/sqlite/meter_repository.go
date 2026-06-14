package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"time"

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
	metadataSchema, err := marshalMetadataSchema(meter.MetadataSchema())
	if err != nil {
		return domainmeter.Meter{}, err
	}

	_, err = r.store.db.ExecContext(ctx, `
INSERT INTO meters (id, name, description, unit, aggregation, metadata_schema, event_retention_days, created_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(id) DO UPDATE SET
	description = excluded.description,
	unit = excluded.unit,
	aggregation = excluded.aggregation,
	metadata_schema = excluded.metadata_schema,
	event_retention_days = excluded.event_retention_days
`, meter.ID(), meter.Name(), meter.Description(), meter.Unit(), string(meter.Aggregation()), metadataSchema, meter.EventRetentionDays(), formatTime(meter.CreatedAt()))
	if err != nil {
		if isUniqueConstraint(err) {
			return domainmeter.Meter{}, errors.Join(domain.ErrConflict, err)
		}
		return domainmeter.Meter{}, err
	}

	return meter, nil
}

func (r *MeterRepository) Find(ctx context.Context, query domainmeter.Query) ([]domainmeter.Meter, error) {
	where := []string{"1 = 1"}
	args := []any{}

	if query.ID != "" {
		where = append(where, "id = ?")
		args = append(args, query.ID)
	}
	if query.Name != "" {
		where = append(where, "name = ?")
		args = append(args, query.Name)
	}
	if query.Cursor != "" {
		where = append(where, "name > ?")
		args = append(args, query.Cursor)
	}
	args = append(args, domainmeter.NormalizeLimit(query.Limit))

	rows, err := r.store.db.QueryContext(ctx, `
SELECT id, name, description, unit, aggregation, metadata_schema, event_retention_days, created_at
FROM meters
WHERE `+strings.Join(where, " AND ")+`
ORDER BY name
LIMIT ?
`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	meters := []domainmeter.Meter{}
	for rows.Next() {
		meter, err := scanMeter(rows)
		if err != nil {
			return nil, err
		}
		meters = append(meters, meter)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return meters, nil
}

func (r *MeterRepository) Count(ctx context.Context) (int, error) {
	var count int
	if err := r.store.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM meters`).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

func (r *MeterRepository) Delete(ctx context.Context, query domainmeter.Query) error {
	meters, err := r.Find(ctx, query)
	if err != nil {
		return err
	}
	if len(meters) == 0 {
		return domain.ErrNotFound
	}

	meter := meters[0]
	var usageCount int
	if err := r.store.db.QueryRowContext(ctx, `
SELECT COUNT(*)
FROM usage_events
WHERE meter_name = ?
`, meter.Name()).Scan(&usageCount); err != nil {
		return err
	}
	if usageCount > 0 {
		return errors.Join(domain.ErrConflict, errors.New("meter has usage"))
	}

	_, err = r.store.db.ExecContext(ctx, `
DELETE FROM meters
WHERE id = ?
`, meter.ID())
	return err
}

func scanMeter(scanner interface {
	Scan(dest ...any) error
}) (domainmeter.Meter, error) {
	var id string
	var name string
	var description string
	var unit string
	var aggregation string
	var metadataSchemaText string
	var eventRetentionDays int
	var createdAtText string

	if err := scanner.Scan(&id, &name, &description, &unit, &aggregation, &metadataSchemaText, &eventRetentionDays, &createdAtText); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domainmeter.Meter{}, domain.ErrNotFound
		}
		return domainmeter.Meter{}, err
	}

	createdAt, err := time.Parse(time.RFC3339Nano, createdAtText)
	if err != nil {
		return domainmeter.Meter{}, err
	}
	metadataSchema, err := unmarshalMetadataSchema(metadataSchemaText)
	if err != nil {
		return domainmeter.Meter{}, err
	}

	return domainmeter.New(id, name, description, unit, domainmeter.Aggregation(aggregation), metadataSchema, eventRetentionDays, createdAt)
}

func marshalMetadataSchema(schema map[string]domainmeter.MetadataType) (string, error) {
	payload := map[string]string{}
	for key, value := range schema {
		payload[key] = string(value)
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func unmarshalMetadataSchema(payload string) (map[string]domainmeter.MetadataType, error) {
	if payload == "" {
		payload = "{}"
	}
	values := map[string]string{}
	if err := json.Unmarshal([]byte(payload), &values); err != nil {
		return nil, err
	}
	schema := map[string]domainmeter.MetadataType{}
	for key, value := range values {
		schema[key] = domainmeter.MetadataType(value)
	}
	return schema, nil
}
