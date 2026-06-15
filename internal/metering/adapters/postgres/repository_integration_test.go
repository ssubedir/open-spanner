package postgres

import (
	"context"
	"errors"
	"math"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	appauth "github.com/ssubedir/open-spanner/internal/auth"
	appusage "github.com/ssubedir/open-spanner/internal/metering/app/usage"
	"github.com/ssubedir/open-spanner/internal/metering/domain"
	domainmeter "github.com/ssubedir/open-spanner/internal/metering/domain/meter"
	domainusage "github.com/ssubedir/open-spanner/internal/metering/domain/usage"
)

func TestIntegrationPostgresAuthRepositoryUserAndSessionFlow(t *testing.T) {
	ctx := context.Background()
	store := newIntegrationStore(t, ctx)
	repo := NewAuthRepository(store)
	now := time.Date(2026, 6, 14, 12, 0, 0, 0, time.UTC)

	user := appauth.User{
		ID:           "user-1",
		Email:        "admin@example.com",
		PasswordHash: "hashed-password",
		CreatedAt:    now,
	}
	if _, err := repo.SaveUser(ctx, user); err != nil {
		t.Fatalf("save user: %v", err)
	}

	found, err := repo.FindUserByEmail(ctx, "admin@example.com")
	if err != nil {
		t.Fatalf("find user: %v", err)
	}
	if found.ID != user.ID || found.PasswordHash != user.PasswordHash {
		t.Fatalf("found user = %#v, want %#v", found, user)
	}

	session := appauth.Session{
		ID:        "session-1",
		UserID:    user.ID,
		TokenHash: appauth.HashToken("session-token"),
		Kind:      appauth.TokenKindAccess,
		CreatedAt: now,
		ExpiresAt: now.Add(time.Hour),
	}
	if _, err := repo.SaveSession(ctx, session); err != nil {
		t.Fatalf("save session: %v", err)
	}

	active, err := repo.FindSessionByTokenHash(ctx, session.TokenHash, appauth.TokenKindAccess, now)
	if err != nil {
		t.Fatalf("find active session: %v", err)
	}
	if active.ID != session.ID || active.UserID != user.ID || active.Kind != appauth.TokenKindAccess {
		t.Fatalf("active session = %#v, want %#v", active, session)
	}

	_, err = repo.FindSessionByTokenHash(ctx, session.TokenHash, appauth.TokenKindRefresh, now)
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("wrong kind session error = %v, want ErrNotFound", err)
	}

	_, err = repo.FindSessionByTokenHash(ctx, session.TokenHash, appauth.TokenKindAccess, now.Add(2*time.Hour))
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expired session error = %v, want ErrNotFound", err)
	}

	apiKey := appauth.APIKey{
		ID:        "key-1",
		UserID:    user.ID,
		Name:      "sdk",
		TokenHash: appauth.HashToken("api-key-token"),
		Prefix:    "osp_sk_test",
		CreatedAt: now,
	}
	if _, err := repo.SaveAPIKey(ctx, apiKey); err != nil {
		t.Fatalf("save api key: %v", err)
	}

	keys, err := repo.ListAPIKeys(ctx, user.ID)
	if err != nil {
		t.Fatalf("list api keys: %v", err)
	}
	if len(keys) != 1 || keys[0].ID != apiKey.ID || keys[0].LastUsedAt != nil {
		t.Fatalf("api keys = %#v", keys)
	}

	foundKey, err := repo.FindAPIKeyByTokenHash(ctx, apiKey.TokenHash)
	if err != nil {
		t.Fatalf("find api key: %v", err)
	}
	if foundKey.ID != apiKey.ID || foundKey.UserID != user.ID {
		t.Fatalf("found api key = %#v", foundKey)
	}

	lastUsedAt := now.Add(time.Minute)
	if err := repo.UpdateAPIKeyLastUsed(ctx, apiKey.ID, lastUsedAt); err != nil {
		t.Fatalf("update api key last used: %v", err)
	}
	usedKey, err := repo.FindAPIKeyByTokenHash(ctx, apiKey.TokenHash)
	if err != nil {
		t.Fatalf("find used api key: %v", err)
	}
	if usedKey.LastUsedAt == nil || !usedKey.LastUsedAt.Equal(lastUsedAt) {
		t.Fatalf("last used at = %#v, want %s", usedKey.LastUsedAt, lastUsedAt)
	}

	if err := repo.DeleteAPIKey(ctx, user.ID, apiKey.ID); err != nil {
		t.Fatalf("delete api key: %v", err)
	}
	_, err = repo.FindAPIKeyByTokenHash(ctx, apiKey.TokenHash)
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("deleted api key error = %v, want ErrNotFound", err)
	}
}

func TestIntegrationPostgresUsageFlow(t *testing.T) {
	ctx := context.Background()
	store := newIntegrationStore(t, ctx)
	meterRepo := NewMeterRepository(store)
	usageRepo := NewUsageRepository(store)

	meter := newIntegrationMeter(t, "meter-1", "api_calls", time.Date(2026, 6, 8, 9, 0, 0, 0, time.UTC))
	if _, err := meterRepo.Save(ctx, meter); err != nil {
		t.Fatalf("save meter: %v", err)
	}

	eventTime := time.Date(2026, 6, 8, 10, 0, 0, 0, time.UTC)
	first := newIntegrationEvent(t, "event-1", "usage-1", "org_123", "api_calls", 3, eventTime, map[string]any{
		"region": "us-east-1",
	})
	saved, err := usageRepo.Save(ctx, first)
	if err != nil {
		t.Fatalf("save usage: %v", err)
	}

	duplicate := newIntegrationEvent(t, "event-2", "usage-1", "org_123", "api_calls", 9, eventTime, map[string]any{
		"region": "us-west-2",
	})
	replayed, err := usageRepo.Save(ctx, duplicate)
	if err != nil {
		t.Fatalf("save duplicate usage: %v", err)
	}
	if replayed.ID() != saved.ID() || replayed.Quantity() != saved.Quantity() {
		t.Fatalf("replayed event = %s/%v, want %s/%v", replayed.ID(), replayed.Quantity(), saved.ID(), saved.Quantity())
	}

	second := newIntegrationEvent(t, "event-3", "", "org_123", "api_calls", 2, eventTime.Add(time.Hour), map[string]any{
		"region": "us-east-1",
	})
	if _, err := usageRepo.Save(ctx, second); err != nil {
		t.Fatalf("save second usage: %v", err)
	}

	filter, err := domainusage.NewFilterCondition("metadata.region", domainusage.FilterOpEqual, "us-east-1", true)
	if err != nil {
		t.Fatalf("new filter: %v", err)
	}
	query, err := domainusage.NewFilteredQuery(
		"org_123",
		"api_calls",
		time.Date(2026, 6, 8, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 6, 9, 0, 0, 0, 0, time.UTC),
		domainusage.BucketDay,
		domainmeter.AggregationSum,
		nil,
		"region",
		0,
		filter,
	)
	if err != nil {
		t.Fatalf("new query: %v", err)
	}

	buckets, err := usageRepo.Query(ctx, query)
	if err != nil {
		t.Fatalf("query usage: %v", err)
	}
	if len(buckets) != 1 {
		t.Fatalf("bucket count = %d, want 1", len(buckets))
	}
	if buckets[0].Quantity() != 5 {
		t.Fatalf("bucket quantity = %v, want 5", buckets[0].Quantity())
	}
	if buckets[0].Group()["region"] != "us-east-1" {
		t.Fatalf("bucket group = %#v, want region us-east-1", buckets[0].Group())
	}
}

func TestIntegrationPostgresBulkReplayAndEventPagination(t *testing.T) {
	ctx := context.Background()
	store := newIntegrationStore(t, ctx)
	meterRepo := NewMeterRepository(store)
	usageRepo := NewUsageRepository(store)

	if _, err := meterRepo.Save(ctx, newIntegrationMeter(t, "meter-1", "tokens", time.Now())); err != nil {
		t.Fatalf("save meter: %v", err)
	}

	events := []domainusage.Event{
		newIntegrationEvent(t, "event-1", "usage-1", "org_123", "tokens", 2, time.Date(2026, 6, 8, 10, 0, 0, 0, time.UTC), nil),
		newIntegrationEvent(t, "event-2", "usage-2", "org_123", "tokens", 3, time.Date(2026, 6, 8, 11, 0, 0, 0, time.UTC), nil),
	}
	first, err := usageRepo.SaveBulk(ctx, "batch-1", events)
	if err != nil {
		t.Fatalf("save bulk: %v", err)
	}

	second, err := usageRepo.SaveBulk(ctx, "batch-1", []domainusage.Event{
		newIntegrationEvent(t, "event-3", "usage-3", "org_123", "tokens", 100, time.Date(2026, 6, 8, 12, 0, 0, 0, time.UTC), nil),
	})
	if err != nil {
		t.Fatalf("replay bulk: %v", err)
	}
	if len(second.Accepted()) != len(first.Accepted()) || len(second.Duplicates()) != 0 {
		t.Fatalf("replayed bulk = %#v, want accepted replay %#v", second, first)
	}

	eventQuery, err := domainusage.NewEventQuery(
		"org_123",
		"tokens",
		time.Date(2026, 6, 8, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 6, 9, 0, 0, 0, 0, time.UTC),
		1,
		domainusage.EventCursor{},
	)
	if err != nil {
		t.Fatalf("new event query: %v", err)
	}

	page, err := usageRepo.FindEvents(ctx, eventQuery)
	if err != nil {
		t.Fatalf("find events: %v", err)
	}
	if len(page.Events()) != 1 || page.NextCursor().IsZero() {
		t.Fatalf("page = %#v, want one event and next cursor", page)
	}
}

func TestIntegrationPostgresSQLAggregationModes(t *testing.T) {
	ctx := context.Background()
	store := newIntegrationStore(t, ctx)
	meterRepo := NewMeterRepository(store)
	usageRepo := NewUsageRepository(store)

	if _, err := meterRepo.Save(ctx, newIntegrationMeter(t, "meter-1", "tokens", time.Now())); err != nil {
		t.Fatalf("save meter: %v", err)
	}

	start := time.Date(2026, 6, 8, 10, 0, 0, 0, time.UTC)
	events := []domainusage.Event{
		newIntegrationEvent(t, "event-1", "", "org_123", "tokens", 10, start.Add(5*time.Minute), nil),
		newIntegrationEvent(t, "event-2", "", "org_123", "tokens", 14, start.Add(15*time.Minute), nil),
		newIntegrationEvent(t, "event-3", "", "org_123", "tokens", 16, start.Add(30*time.Minute), nil),
	}
	for _, event := range events {
		if _, err := usageRepo.Save(ctx, event); err != nil {
			t.Fatalf("save usage %s: %v", event.ID(), err)
		}
	}

	tests := []struct {
		name        string
		aggregation domainmeter.Aggregation
		want        float64
	}{
		{"sum", domainmeter.AggregationSum, 40},
		{"count", domainmeter.AggregationCount, 3},
		{"avg", domainmeter.AggregationAverage, 40.0 / 3.0},
		{"min", domainmeter.AggregationMinimum, 10},
		{"max", domainmeter.AggregationMaximum, 16},
		{"first", domainmeter.AggregationFirst, 10},
		{"last", domainmeter.AggregationLast, 16},
		{"rate", domainmeter.AggregationRate, 3.0 / 3600.0},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			query, err := domainusage.NewQuery(
				"org_123",
				"tokens",
				start,
				start.Add(time.Hour),
				domainusage.BucketHour,
				tc.aggregation,
				nil,
				"",
				0,
			)
			if err != nil {
				t.Fatalf("new query: %v", err)
			}
			buckets, err := usageRepo.Query(ctx, query)
			if err != nil {
				t.Fatalf("query usage: %v", err)
			}
			if len(buckets) != 1 {
				t.Fatalf("bucket count = %d, want 1", len(buckets))
			}
			if math.Abs(buckets[0].Quantity()-tc.want) > 0.000001 {
				t.Fatalf("bucket quantity = %v, want %v", buckets[0].Quantity(), tc.want)
			}
		})
	}
}

func TestIntegrationPostgresSQLAggregationMetadataFilters(t *testing.T) {
	ctx := context.Background()
	store := newIntegrationStore(t, ctx)
	meterRepo := NewMeterRepository(store)
	usageRepo := NewUsageRepository(store)

	if _, err := meterRepo.Save(ctx, newIntegrationMeter(t, "meter-1", "requests", time.Now())); err != nil {
		t.Fatalf("save meter: %v", err)
	}

	start := time.Date(2026, 6, 8, 10, 0, 0, 0, time.UTC)
	events := []domainusage.Event{
		newIntegrationEvent(t, "event-1", "", "org_123", "requests", 2, start.Add(5*time.Minute), map[string]any{"region": "us-east-1"}),
		newIntegrationEvent(t, "event-2", "", "org_123", "requests", 3, start.Add(15*time.Minute), map[string]any{"region": "us-west-2"}),
		newIntegrationEvent(t, "event-3", "", "org_123", "requests", 5, start.Add(30*time.Minute), map[string]any{"region": "us-east-1"}),
	}
	for _, event := range events {
		if _, err := usageRepo.Save(ctx, event); err != nil {
			t.Fatalf("save usage %s: %v", event.ID(), err)
		}
	}

	query, err := domainusage.NewQuery(
		"org_123",
		"requests",
		start,
		start.Add(time.Hour),
		domainusage.BucketHour,
		domainmeter.AggregationSum,
		map[string]string{"region": "us-east-1"},
		"region",
		0,
	)
	if err != nil {
		t.Fatalf("new query: %v", err)
	}
	buckets, err := usageRepo.Query(ctx, query)
	if err != nil {
		t.Fatalf("query usage: %v", err)
	}
	if len(buckets) != 1 || buckets[0].Quantity() != 7 || buckets[0].Group()["region"] != "us-east-1" {
		t.Fatalf("filtered buckets = %#v", buckets)
	}
}

func TestIntegrationPostgresPruneEventsDeletesInBatches(t *testing.T) {
	ctx := context.Background()
	store := newIntegrationStore(t, ctx)
	meterRepo := NewMeterRepository(store)
	usageRepo := NewUsageRepository(store)

	if _, err := meterRepo.Save(ctx, newIntegrationMeter(t, "meter-1", "retained", time.Now())); err != nil {
		t.Fatalf("save meter: %v", err)
	}

	before := time.Date(2026, 6, 9, 0, 0, 0, 0, time.UTC)
	for i := 0; i < pruneDeleteBatchSize+5; i++ {
		event := newIntegrationEvent(t, "old-event-"+strconv.Itoa(i), "", "org_123", "retained", 1, before.Add(-time.Duration(i+1)*time.Minute), nil)
		if _, err := usageRepo.Save(ctx, event); err != nil {
			t.Fatalf("save old usage %d: %v", i, err)
		}
	}
	if _, err := usageRepo.Save(ctx, newIntegrationEvent(t, "new-event", "", "org_123", "retained", 2, before.Add(time.Hour), nil)); err != nil {
		t.Fatalf("save retained usage: %v", err)
	}

	pruneQuery, err := domainusage.NewPruneQuery("retained", before)
	if err != nil {
		t.Fatalf("new prune query: %v", err)
	}
	deleted, err := usageRepo.PruneEvents(ctx, pruneQuery)
	if err != nil {
		t.Fatalf("prune events: %v", err)
	}
	if deleted != pruneDeleteBatchSize+5 {
		t.Fatalf("deleted = %d, want %d", deleted, pruneDeleteBatchSize+5)
	}

	count, err := usageRepo.CountEvents(ctx)
	if err != nil {
		t.Fatalf("count events: %v", err)
	}
	if count != 1 {
		t.Fatalf("event count = %d, want retained event only", count)
	}
}

func TestIntegrationPostgresPruneServiceUsesAdvisoryLock(t *testing.T) {
	ctx := context.Background()
	store := newIntegrationStore(t, ctx)
	meterRepo := NewMeterRepository(store)
	usageRepo := NewUsageRepository(store)

	if _, err := meterRepo.Save(ctx, newIntegrationMeter(t, "meter-1", "locked", time.Now())); err != nil {
		t.Fatalf("save meter: %v", err)
	}

	service := appusage.NewService(meterRepo, usageRepo, store)
	err := store.WithinTransaction(ctx, func(txCtx context.Context) error {
		locked, err := usageRepo.TryPruneLock(txCtx)
		if err != nil {
			return err
		}
		if !locked {
			t.Fatal("initial prune lock = false, want true")
		}

		_, err = service.PruneEvents(ctx, appusage.PruneCommand{})
		if !errors.Is(err, domain.ErrConflict) {
			t.Fatalf("prune error = %v, want ErrConflict", err)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("lock transaction: %v", err)
	}
}

func newIntegrationStore(t *testing.T, ctx context.Context) *Store {
	t.Helper()

	dsn := os.Getenv("OPEN_SPANNER_TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("set OPEN_SPANNER_TEST_POSTGRES_DSN to run Postgres integration tests")
	}

	store, err := NewStore(ctx, dsn)
	if err != nil {
		t.Fatalf("new postgres store: %v", err)
	}
	t.Cleanup(func() {
		cleanIntegrationStore(t, ctx, store)
		if err := store.Close(); err != nil {
			t.Fatalf("close postgres store: %v", err)
		}
	})

	cleanIntegrationStore(t, ctx, store)
	return store
}

func cleanIntegrationStore(t *testing.T, ctx context.Context, store *Store) {
	t.Helper()

	_, err := store.db.ExecContext(ctx, `
TRUNCATE TABLE auth_sessions, auth_users, usage_ingestions, usage_prune_runs, bulk_usage_ingestions, usage_events, meters RESTART IDENTITY CASCADE
`)
	if err != nil {
		t.Fatalf("clean postgres store: %v", err)
	}
}

func newIntegrationMeter(t *testing.T, id string, name string, createdAt time.Time) domainmeter.Meter {
	t.Helper()

	meter, err := domainmeter.New(id, name, "integration meter", "count", domainmeter.AggregationSum, map[string]domainmeter.MetadataType{}, 0, createdAt)
	if err != nil {
		t.Fatalf("new meter: %v", err)
	}
	return meter
}

func newIntegrationEvent(t *testing.T, id string, idempotencyKey string, subject string, meterName string, quantity float64, eventTime time.Time, metadata map[string]any) domainusage.Event {
	t.Helper()

	if metadata == nil {
		metadata = map[string]any{}
	}
	for key, value := range map[string]any{"source": "integration"} {
		if _, exists := metadata[key]; !exists {
			metadata[key] = value
		}
	}

	event, err := domainusage.NewEvent(
		id,
		idempotencyKey,
		subject,
		meterName,
		quantity,
		eventTime,
		eventTime.Add(time.Second),
		metadata,
	)
	if err != nil {
		t.Fatalf("new usage event: %v", err)
	}
	return event
}

func TestIntegrationPostgresJSONPathRejectsUnsafeKeys(t *testing.T) {
	unsafeInputs := []string{
		"metadata.region'); DROP TABLE meters; --",
		"metadata.region-name",
		"metadata.region name",
	}

	for _, input := range unsafeInputs {
		t.Run(strings.ReplaceAll(input, " ", "_"), func(t *testing.T) {
			if _, err := filterFieldSQL(input); err == nil {
				t.Fatalf("filterFieldSQL(%q) error = nil, want rejection", input)
			}
		})
	}
}

func TestIntegrationPostgresStoreTracksMigrations(t *testing.T) {
	ctx := context.Background()
	store := newIntegrationStore(t, ctx)

	var version int
	var dirty bool
	err := store.db.QueryRowContext(ctx, `
SELECT version, dirty
FROM schema_migrations
LIMIT 1
`).Scan(&version, &dirty)
	if err != nil {
		t.Fatalf("query schema migration version: %v", err)
	}

	if version != 8 || dirty {
		t.Fatalf("schema migration version = %d dirty=%v, want version 8 dirty=false", version, dirty)
	}
}

func TestIntegrationPostgresStoreCreatesUsagePerformanceIndexes(t *testing.T) {
	ctx := context.Background()
	store := newIntegrationStore(t, ctx)

	want := map[string]bool{
		"idx_usage_events_subject_meter_time_quantity": false,
		"idx_usage_events_prune_meter_time_id":         false,
		"idx_usage_events_meter_stats":                 false,
		"idx_usage_events_subject_stats":               false,
		"idx_usage_events_metadata_gin":                false,
	}

	rows, err := store.db.QueryContext(ctx, `
SELECT indexname
FROM pg_indexes
WHERE schemaname = current_schema()
	AND tablename = 'usage_events'
`)
	if err != nil {
		t.Fatalf("query postgres indexes: %v", err)
	}
	defer rows.Close()

	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatalf("scan postgres index: %v", err)
		}
		if _, ok := want[name]; ok {
			want[name] = true
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("postgres index rows: %v", err)
	}

	for name, found := range want {
		if !found {
			t.Fatalf("missing postgres usage_events index %s", name)
		}
	}
}

func TestIntegrationPostgresStoreUsesJSONBMetadata(t *testing.T) {
	ctx := context.Background()
	store := newIntegrationStore(t, ctx)

	var dataType string
	err := store.db.QueryRowContext(ctx, `
SELECT data_type
FROM information_schema.columns
WHERE table_schema = current_schema()
	AND table_name = 'usage_events'
	AND column_name = 'metadata'
`).Scan(&dataType)
	if err != nil {
		t.Fatalf("query usage metadata column: %v", err)
	}

	if dataType != "jsonb" {
		t.Fatalf("usage metadata data type = %s, want jsonb", dataType)
	}
}
