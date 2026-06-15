package meter

import (
	"context"
	"errors"
	"testing"

	"github.com/ssubedir/open-spanner/internal/config"
	"github.com/ssubedir/open-spanner/internal/metering/adapters/sqlite"
	"github.com/ssubedir/open-spanner/internal/metering/domain"
	domainmeter "github.com/ssubedir/open-spanner/internal/metering/domain/meter"
)

func TestServiceCreateListAndGet(t *testing.T) {
	ctx := context.Background()
	service := newTestService()

	created, err := service.Create(ctx, CreateCommand{
		Name:        "api_calls",
		Description: "API calls",
		Unit:        "call",
		Aggregation: domainmeter.AggregationSum,
		Dimensions: []domainmeter.Dimension{
			mustDimension(t, "region", domainmeter.MetadataString, "Region", "Deployment region", true),
		},
	})
	if err != nil {
		t.Fatalf("create meter: %v", err)
	}
	if created.ID == "" {
		t.Fatal("created meter id is empty")
	}
	if created.MetadataSchema["region"] != "string" {
		t.Fatalf("metadata schema = %#v", created.MetadataSchema)
	}
	if len(created.Dimensions) != 1 || created.Dimensions[0].DisplayName != "Region" || created.Dimensions[0].Description != "Deployment region" || !created.Dimensions[0].Required {
		t.Fatalf("dimensions = %#v", created.Dimensions)
	}
	if created.EventRetentionDays != domainmeter.DefaultEventRetentionDays {
		t.Fatalf("event retention days = %d, want %d", created.EventRetentionDays, domainmeter.DefaultEventRetentionDays)
	}

	listed, err := service.List(ctx, ListQuery{})
	if err != nil {
		t.Fatalf("list meters: %v", err)
	}
	if len(listed.Items) != 1 {
		t.Fatalf("listed meter count = %d, want 1", len(listed.Items))
	}

	fetched, err := service.Get(ctx, GetQuery{ID: created.ID})
	if err != nil {
		t.Fatalf("get meter: %v", err)
	}
	if fetched.ID != created.ID || fetched.Name != "api_calls" {
		t.Fatalf("fetched meter = %#v, created = %#v", fetched, created)
	}
}

func TestServiceListCanFilterByName(t *testing.T) {
	ctx := context.Background()
	service := newTestService()

	if _, err := service.Create(ctx, CreateCommand{Name: "api_calls", Unit: "call"}); err != nil {
		t.Fatalf("create api_calls meter: %v", err)
	}
	if _, err := service.Create(ctx, CreateCommand{Name: "tokens", Unit: "token"}); err != nil {
		t.Fatalf("create tokens meter: %v", err)
	}

	listed, err := service.List(ctx, ListQuery{Name: "tokens"})
	if err != nil {
		t.Fatalf("list filtered meters: %v", err)
	}
	if len(listed.Items) != 1 || listed.Items[0].Name != "tokens" {
		t.Fatalf("filtered meters = %#v", listed)
	}
}

func TestServiceCreateStoresEventRetentionDays(t *testing.T) {
	ctx := context.Background()
	service := newTestService()

	created, err := service.Create(ctx, CreateCommand{
		Name:               "api_calls",
		Unit:               "call",
		EventRetentionDays: 30,
	})
	if err != nil {
		t.Fatalf("create meter: %v", err)
	}
	if created.EventRetentionDays != 30 {
		t.Fatalf("event retention days = %d, want 30", created.EventRetentionDays)
	}
}

func TestServiceGetMissingReturnsNotFound(t *testing.T) {
	_, err := newTestService().Get(context.Background(), GetQuery{ID: "missing"})
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("get missing error = %v, want ErrNotFound", err)
	}
}

func TestServiceUpdateDescription(t *testing.T) {
	ctx := context.Background()
	service := newTestService()

	created, err := service.Create(ctx, CreateCommand{
		Name:        "api_calls",
		Description: "Old description",
		Unit:        "call",
	})
	if err != nil {
		t.Fatalf("create meter: %v", err)
	}

	description := "New description"
	updated, err := service.Update(ctx, UpdateCommand{
		ID:          created.ID,
		Description: &description,
	})
	if err != nil {
		t.Fatalf("update meter: %v", err)
	}
	if updated.Description != "New description" {
		t.Fatalf("updated description = %q, want New description", updated.Description)
	}
	if updated.Name != created.Name || updated.Unit != created.Unit {
		t.Fatalf("update changed immutable fields: %#v", updated)
	}
}

func TestServiceUpdateDefinitionSettings(t *testing.T) {
	ctx := context.Background()
	service := newTestService()

	created, err := service.Create(ctx, CreateCommand{
		Name:               "api_calls",
		Description:        "Old description",
		Unit:               "call",
		Aggregation:        domainmeter.AggregationSum,
		EventRetentionDays: 30,
		MetadataSchema: map[string]domainmeter.MetadataType{
			"region": domainmeter.MetadataString,
		},
	})
	if err != nil {
		t.Fatalf("create meter: %v", err)
	}

	description := "Updated description"
	unit := "request"
	aggregation := domainmeter.AggregationCount
	retention := 365
	metadataSchema := map[string]domainmeter.MetadataType{
		"plan": domainmeter.MetadataString,
	}
	updated, err := service.Update(ctx, UpdateCommand{
		ID:                 created.ID,
		Description:        &description,
		Unit:               &unit,
		Aggregation:        &aggregation,
		EventRetentionDays: &retention,
		MetadataSchema:     &metadataSchema,
	})
	if err != nil {
		t.Fatalf("update meter: %v", err)
	}
	if updated.Name != created.Name {
		t.Fatalf("updated name = %q, want %q", updated.Name, created.Name)
	}
	if updated.Description != description || updated.Unit != unit || updated.Aggregation != string(aggregation) || updated.EventRetentionDays != retention {
		t.Fatalf("updated meter = %#v", updated)
	}
	if updated.MetadataSchema["plan"] != string(domainmeter.MetadataString) || updated.MetadataSchema["region"] != "" {
		t.Fatalf("updated metadata schema = %#v", updated.MetadataSchema)
	}
}

func TestServiceUpdateDimensions(t *testing.T) {
	ctx := context.Background()
	service := newTestService()

	created, err := service.Create(ctx, CreateCommand{
		Name: "api_calls",
		Unit: "call",
		Dimensions: []domainmeter.Dimension{
			mustDimension(t, "region", domainmeter.MetadataString, "Region", "", true),
		},
	})
	if err != nil {
		t.Fatalf("create meter: %v", err)
	}

	dimensions := []domainmeter.Dimension{
		mustDimension(t, "status", domainmeter.MetadataNumber, "HTTP status", "Response status code", false),
	}
	updated, err := service.Update(ctx, UpdateCommand{
		ID:         created.ID,
		Dimensions: &dimensions,
	})
	if err != nil {
		t.Fatalf("update meter dimensions: %v", err)
	}
	if len(updated.Dimensions) != 1 || updated.Dimensions[0].Name != "status" || updated.Dimensions[0].Type != "number" || updated.Dimensions[0].Required {
		t.Fatalf("updated dimensions = %#v", updated.Dimensions)
	}
	if updated.MetadataSchema["status"] != "number" || updated.MetadataSchema["region"] != "" {
		t.Fatalf("updated metadata schema = %#v", updated.MetadataSchema)
	}
}

func TestServiceDelete(t *testing.T) {
	ctx := context.Background()
	service := newTestService()

	created, err := service.Create(ctx, CreateCommand{Name: "api_calls", Unit: "call"})
	if err != nil {
		t.Fatalf("create meter: %v", err)
	}
	if err := service.Delete(ctx, DeleteCommand{ID: created.ID}); err != nil {
		t.Fatalf("delete meter: %v", err)
	}

	_, err = service.Get(ctx, GetQuery{ID: created.ID})
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("get deleted meter error = %v, want ErrNotFound", err)
	}
}

func TestServiceCreateInvalidMeterReturnsInvalidInput(t *testing.T) {
	_, err := newTestService().Create(context.Background(), CreateCommand{Name: "", Unit: "call"})
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("create invalid error = %v, want ErrInvalidInput", err)
	}
}

func TestServiceCreateInvalidMetadataSchemaReturnsInvalidInput(t *testing.T) {
	_, err := newTestService().Create(context.Background(), CreateCommand{
		Name: "api_calls",
		Unit: "call",
		MetadataSchema: map[string]domainmeter.MetadataType{
			"region": domainmeter.MetadataType("object"),
		},
	})
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("create invalid metadata schema error = %v, want ErrInvalidInput", err)
	}
}

func TestServiceCreateInvalidEventRetentionDaysReturnsInvalidInput(t *testing.T) {
	_, err := newTestService().Create(context.Background(), CreateCommand{
		Name:               "api_calls",
		Unit:               "call",
		EventRetentionDays: -1,
	})
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("create invalid retention error = %v, want ErrInvalidInput", err)
	}
}

func newTestService() Service {
	store, err := sqlite.NewStore(context.Background(), ":memory:", config.DBPoolConfig{MaxOpenConns: 1})
	if err != nil {
		panic(err)
	}
	return NewService(sqlite.NewMeterRepository(store))
}

func mustDimension(t *testing.T, name string, metadataType domainmeter.MetadataType, displayName string, description string, required bool) domainmeter.Dimension {
	t.Helper()

	dimension, err := domainmeter.NewDimension(name, metadataType, displayName, description, required)
	if err != nil {
		t.Fatalf("new dimension: %v", err)
	}
	return dimension
}
