package meter

import (
	"context"
	"errors"
	"testing"

	"github.com/ssubedir/open-spanner/internal/metering/adapters/memory"
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
		MetadataSchema: map[string]domainmeter.MetadataType{
			"region": domainmeter.MetadataString,
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

	updated, err := service.Update(ctx, UpdateCommand{
		ID:          created.ID,
		Description: "New description",
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
	store := memory.NewStore()
	return NewService(memory.NewMeterRepository(store))
}
