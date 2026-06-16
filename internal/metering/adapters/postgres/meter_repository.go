package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"github.com/ssubedir/open-spanner/internal/metering/adapters/postgres/postgresdb"
	"github.com/ssubedir/open-spanner/internal/metering/domain"
	domainmeter "github.com/ssubedir/open-spanner/internal/metering/domain/meter"
)

type MeterRepository struct {
	store   *Store
	queries *postgresdb.Queries
}

func NewMeterRepository(store *Store) *MeterRepository {
	return &MeterRepository{store: store, queries: postgresdb.New(store)}
}

func (r *MeterRepository) Save(ctx context.Context, meter domainmeter.Meter) (domainmeter.Meter, error) {
	metadataSchema, err := marshalMetadataSchema(meter.MetadataSchema())
	if err != nil {
		return domainmeter.Meter{}, err
	}
	dimensions, err := marshalDimensions(meter.Dimensions())
	if err != nil {
		return domainmeter.Meter{}, err
	}

	err = r.queries.SaveMeter(ctx, postgresdb.SaveMeterParams{
		ID:                 meter.ID(),
		Name:               meter.Name(),
		Description:        meter.Description(),
		Unit:               meter.Unit(),
		Aggregation:        string(meter.Aggregation()),
		MetadataSchema:     metadataSchema,
		Dimensions:         dimensions,
		EventRetentionDays: int32(meter.EventRetentionDays()),
		CreatedAt:          formatTime(meter.CreatedAt()),
	})
	if err != nil {
		if isUniqueConstraint(err) {
			return domainmeter.Meter{}, errors.Join(domain.ErrConflict, err)
		}
		return domainmeter.Meter{}, err
	}

	return meter, nil
}

func (r *MeterRepository) Find(ctx context.Context, query domainmeter.Query) ([]domainmeter.Meter, error) {
	rows, err := r.queries.ListMeters(ctx, postgresdb.ListMetersParams{
		ID:     optionalString(query.ID),
		Name:   optionalString(query.Name),
		Cursor: optionalString(query.Cursor),
		Limit:  int32(domainmeter.NormalizeLimit(query.Limit)),
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
			row.MetadataSchema,
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
	count, err := r.queries.CountMeters(ctx)
	if err != nil {
		return 0, err
	}
	return int(count), nil
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
	usageCount, err := r.queries.CountUsageEventsForMeter(ctx, meter.Name())
	if err != nil {
		return err
	}
	if usageCount > 0 {
		return errors.Join(domain.ErrConflict, errors.New("meter has usage"))
	}

	return r.queries.DeleteMeter(ctx, meter.ID())
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
	var dimensionsText string
	var eventRetentionDays int
	var createdAtText string

	if err := scanner.Scan(&id, &name, &description, &unit, &aggregation, &metadataSchemaText, &dimensionsText, &eventRetentionDays, &createdAtText); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domainmeter.Meter{}, domain.ErrNotFound
		}
		return domainmeter.Meter{}, err
	}

	return meterFromFields(id, name, description, unit, aggregation, metadataSchemaText, dimensionsText, eventRetentionDays, createdAtText)
}

func meterFromFields(id string, name string, description string, unit string, aggregation string, metadataSchemaText string, dimensionsText string, eventRetentionDays int, createdAtText string) (domainmeter.Meter, error) {
	createdAt, err := time.Parse(time.RFC3339Nano, createdAtText)
	if err != nil {
		return domainmeter.Meter{}, err
	}
	metadataSchema, err := unmarshalMetadataSchema(metadataSchemaText)
	if err != nil {
		return domainmeter.Meter{}, err
	}
	dimensions, err := unmarshalDimensions(dimensionsText, metadataSchema)
	if err != nil {
		return domainmeter.Meter{}, err
	}

	return domainmeter.NewWithDimensions(id, name, description, unit, domainmeter.Aggregation(aggregation), dimensions, eventRetentionDays, createdAt)
}

func optionalString(value string) sql.NullString {
	if value == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: value, Valid: true}
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

func unmarshalDimensions(payload string, fallbackSchema map[string]domainmeter.MetadataType) ([]domainmeter.Dimension, error) {
	if payload == "" {
		payload = "[]"
	}
	values := []dimensionPayload{}
	if err := json.Unmarshal([]byte(payload), &values); err != nil {
		return nil, err
	}
	if len(values) == 0 {
		return domainmeter.DimensionsFromMetadataSchema(fallbackSchema)
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
