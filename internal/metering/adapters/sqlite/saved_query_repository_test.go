package sqlite

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	appauth "github.com/ssubedir/open-spanner/internal/auth"
	"github.com/ssubedir/open-spanner/internal/config"
	appsavedquery "github.com/ssubedir/open-spanner/internal/metering/app/savedquery"
	"github.com/ssubedir/open-spanner/internal/metering/domain"
)

func TestSavedQueryRepositoryCRUD(t *testing.T) {
	ctx := context.Background()
	store, err := NewStore(ctx, ":memory:", config.DBPoolConfig{MaxOpenConns: 1})
	if err != nil {
		t.Fatalf("new sqlite store: %v", err)
	}
	t.Cleanup(func() {
		if err := store.Close(); err != nil {
			t.Fatalf("close store: %v", err)
		}
	})

	authRepo := NewAuthRepository(store)
	user := appauth.User{ID: "user-1", Email: "admin@example.com", PasswordHash: "hash", CreatedAt: time.Now().UTC()}
	if _, err := authRepo.SaveUser(ctx, user); err != nil {
		t.Fatalf("save auth user: %v", err)
	}

	repo := NewSavedQueryRepository(store)
	query := appsavedquery.SavedQuery{
		ID:         "query-1",
		UserID:     user.ID,
		Name:       "Usage by endpoint",
		Query:      json.RawMessage(`{"combinator":"and","rules":[]}`),
		GroupBy:    []string{"endpoint"},
		BucketSize: "day",
		Limit:      500,
		Pinned:     true,
		Position:   2,
		CreatedAt:  time.Date(2026, 6, 15, 12, 0, 0, 0, time.UTC),
		UpdatedAt:  time.Date(2026, 6, 15, 12, 0, 0, 0, time.UTC),
	}
	if _, err := repo.Save(ctx, query); err != nil {
		t.Fatalf("save query: %v", err)
	}

	list, err := repo.Find(ctx, appsavedquery.FindQuery{UserID: user.ID})
	if err != nil {
		t.Fatalf("find queries: %v", err)
	}
	if len(list) != 1 || list[0].Name != query.Name || list[0].GroupBy[0] != "endpoint" || !list[0].Pinned || list[0].Position != 2 {
		t.Fatalf("queries = %#v", list)
	}

	query.Name = "Usage by status"
	query.GroupBy = []string{"status"}
	query.Pinned = false
	query.Position = 0
	query.UpdatedAt = query.UpdatedAt.Add(time.Hour)
	if _, err := repo.Save(ctx, query); err != nil {
		t.Fatalf("update query: %v", err)
	}
	byID, err := repo.Find(ctx, appsavedquery.FindQuery{UserID: user.ID, ID: query.ID})
	if err != nil {
		t.Fatalf("find query by id: %v", err)
	}
	if len(byID) != 1 || byID[0].Name != "Usage by status" || byID[0].GroupBy[0] != "status" || byID[0].Pinned || byID[0].Position != 0 {
		t.Fatalf("query by id = %#v", byID)
	}

	if err := repo.Delete(ctx, user.ID, query.ID); err != nil {
		t.Fatalf("delete query: %v", err)
	}
	if err := repo.Delete(ctx, user.ID, query.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("delete missing query error = %v, want ErrNotFound", err)
	}
}
