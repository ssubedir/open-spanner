package meter

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/ssubedir/open-spanner/internal/config"
	"github.com/ssubedir/open-spanner/internal/metering/adapters/sqlite"
	"github.com/ssubedir/open-spanner/internal/metering/domain"
	domainmeter "github.com/ssubedir/open-spanner/internal/metering/domain/meter"
	domainusage "github.com/ssubedir/open-spanner/internal/metering/domain/usage"
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
		Dimensions: []domainmeter.Dimension{
			mustDimension(t, "region", domainmeter.MetadataString, "Region", "", true),
		},
	})
	if err != nil {
		t.Fatalf("create meter: %v", err)
	}

	description := "Updated description"
	unit := "request"
	aggregation := domainmeter.AggregationCount
	retention := 365
	dimensions := []domainmeter.Dimension{
		mustDimension(t, "plan", domainmeter.MetadataString, "Plan", "", true),
	}
	updated, err := service.Update(ctx, UpdateCommand{
		ID:                 created.ID,
		Description:        &description,
		Unit:               &unit,
		Aggregation:        &aggregation,
		EventRetentionDays: &retention,
		Dimensions:         &dimensions,
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
	if len(updated.Dimensions) != 1 || updated.Dimensions[0].Name != "plan" || updated.Dimensions[0].Type != string(domainmeter.MetadataString) {
		t.Fatalf("updated dimensions = %#v", updated.Dimensions)
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
}

func TestServiceUpdateDimensionsAllowsSafeChangesWithUsage(t *testing.T) {
	ctx := context.Background()
	service, usageRepo := newTestServiceWithUsage(t, ctx)

	created, err := service.Create(ctx, CreateCommand{
		Name: "api_calls",
		Unit: "call",
		Dimensions: []domainmeter.Dimension{
			mustDimension(t, "region", domainmeter.MetadataString, "Region", "", true),
			mustDimension(t, "status", domainmeter.MetadataNumber, "Status", "", false),
		},
	})
	if err != nil {
		t.Fatalf("create meter: %v", err)
	}
	recordUsage(t, ctx, usageRepo, created.Name, map[string]any{"region": "us-east", "status": 200})

	dimensions := []domainmeter.Dimension{
		mustDimension(t, "region", domainmeter.MetadataString, "Serving region", "Updated display metadata", true, true),
		mustDimension(t, "status", domainmeter.MetadataNumber, "HTTP status", "Response status code", false),
		mustDimension(t, "legacy", domainmeter.MetadataString, "Legacy", "Deprecated required dimension", true, true),
		mustDimension(t, "plan", domainmeter.MetadataString, "Plan", "Optional billing plan", false),
	}
	updated, err := service.Update(ctx, UpdateCommand{
		ID:         created.ID,
		Dimensions: &dimensions,
	})
	if err != nil {
		t.Fatalf("update safe dimensions: %v", err)
	}
	if len(updated.Dimensions) != 4 || updated.Dimensions[0].Name != "region" || !updated.Dimensions[0].Required || !updated.Dimensions[0].Deprecated {
		t.Fatalf("updated dimensions = %#v", updated.Dimensions)
	}
	if !updated.Dimensions[2].Deprecated {
		t.Fatalf("deprecated dimension not preserved: %#v", updated.Dimensions)
	}
}

func TestServiceUpdateDimensionsRejectsUnsafeChangesWithUsage(t *testing.T) {
	ctx := context.Background()

	for name, dimensions := range map[string][]domainmeter.Dimension{
		"remove dimension": {
			mustDimension(t, "region", domainmeter.MetadataString, "Region", "", true),
		},
		"rename dimension": {
			mustDimension(t, "region", domainmeter.MetadataString, "Region", "", true),
			mustDimension(t, "code", domainmeter.MetadataNumber, "Status", "", false),
		},
		"change type": {
			mustDimension(t, "region", domainmeter.MetadataString, "Region", "", true),
			mustDimension(t, "status", domainmeter.MetadataString, "Status", "", false),
		},
		"make optional dimension required": {
			mustDimension(t, "region", domainmeter.MetadataString, "Region", "", true),
			mustDimension(t, "status", domainmeter.MetadataNumber, "Status", "", true),
		},
		"add required dimension": {
			mustDimension(t, "region", domainmeter.MetadataString, "Region", "", true),
			mustDimension(t, "status", domainmeter.MetadataNumber, "Status", "", false),
			mustDimension(t, "plan", domainmeter.MetadataString, "Plan", "", true),
		},
	} {
		t.Run(name, func(t *testing.T) {
			service, usageRepo := newTestServiceWithUsage(t, ctx)
			created, err := service.Create(ctx, CreateCommand{
				Name: "api_calls",
				Unit: "call",
				Dimensions: []domainmeter.Dimension{
					mustDimension(t, "region", domainmeter.MetadataString, "Region", "", true),
					mustDimension(t, "status", domainmeter.MetadataNumber, "Status", "", false),
				},
			})
			if err != nil {
				t.Fatalf("create meter: %v", err)
			}
			recordUsage(t, ctx, usageRepo, created.Name, map[string]any{"region": "us-east", "status": 200})

			_, err = service.Update(ctx, UpdateCommand{
				ID:         created.ID,
				Dimensions: &dimensions,
			})
			if !errors.Is(err, domain.ErrConflict) {
				t.Fatalf("update unsafe dimensions error = %v, want ErrConflict", err)
			}
		})
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

func TestServiceCreateDuplicateDimensionReturnsInvalidInput(t *testing.T) {
	_, err := newTestService().Create(context.Background(), CreateCommand{
		Name: "api_calls",
		Unit: "call",
		Dimensions: []domainmeter.Dimension{
			mustDimension(t, "region", domainmeter.MetadataString, "Region", "", true),
			mustDimension(t, "region", domainmeter.MetadataString, "Region", "", true),
		},
	})
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("create invalid dimension error = %v, want ErrInvalidInput", err)
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

func newTestServiceWithUsage(t *testing.T, ctx context.Context) (Service, *sqlite.UsageRepository) {
	t.Helper()

	store, err := sqlite.NewStore(ctx, ":memory:", config.DBPoolConfig{MaxOpenConns: 1})
	if err != nil {
		t.Fatalf("new sqlite store: %v", err)
	}
	usageRepo := sqlite.NewUsageRepository(store)
	return NewService(sqlite.NewMeterRepository(store), usageRepo), usageRepo
}

func recordUsage(t *testing.T, ctx context.Context, usageRepo *sqlite.UsageRepository, meterName string, metadata map[string]any) {
	t.Helper()

	event, err := domainusage.NewEvent(
		"usage-1",
		"",
		"org_123",
		meterName,
		1,
		time.Date(2026, 6, 8, 12, 0, 0, 0, time.UTC),
		time.Date(2026, 6, 8, 12, 0, 1, 0, time.UTC),
		metadata,
	)
	if err != nil {
		t.Fatalf("new usage event: %v", err)
	}
	if _, err := usageRepo.Save(ctx, event); err != nil {
		t.Fatalf("save usage event: %v", err)
	}
}

func mustDimension(t *testing.T, name string, metadataType domainmeter.MetadataType, displayName string, description string, required bool, deprecated ...bool) domainmeter.Dimension {
	t.Helper()

	dimension, err := domainmeter.NewDimension(name, metadataType, displayName, description, required, deprecated...)
	if err != nil {
		t.Fatalf("new dimension: %v", err)
	}
	return dimension
}
