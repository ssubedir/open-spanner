package savedquery

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/ssubedir/open-spanner/internal/metering/domain"
)

func TestServiceCreatesListsUpdatesAndDeletesSavedQuery(t *testing.T) {
	repo := newFakeRepository()
	service := NewService(repo).(*service)
	now := time.Date(2026, 6, 15, 12, 0, 0, 0, time.UTC)
	service.now = func() time.Time { return now }

	created, err := service.Create(context.Background(), SaveCommand{
		UserID:     "user-1",
		Name:       " API usage by endpoint ",
		Query:      json.RawMessage(`{"combinator":"and","rules":[]}`),
		GroupBy:    []string{"endpoint", "endpoint", "status"},
		BucketSize: "hour",
		Limit:      200,
	})
	if err != nil {
		t.Fatalf("create saved query: %v", err)
	}
	if created.ID == "" || created.Name != "API usage by endpoint" || created.BucketSize != "hour" || created.Limit != 200 {
		t.Fatalf("created saved query = %#v", created)
	}
	if len(created.GroupBy) != 2 || created.GroupBy[0] != "endpoint" || created.GroupBy[1] != "status" {
		t.Fatalf("created group_by = %#v", created.GroupBy)
	}

	list, err := service.List(context.Background(), ListQuery{UserID: "user-1"})
	if err != nil {
		t.Fatalf("list saved queries: %v", err)
	}
	if len(list.Items) != 1 || list.Items[0].ID != created.ID {
		t.Fatalf("saved query list = %#v", list)
	}

	later := now.Add(time.Hour)
	service.now = func() time.Time { return later }
	updated, err := service.Update(context.Background(), UpdateCommand{
		ID:         created.ID,
		UserID:     "user-1",
		Name:       "Status errors",
		Query:      json.RawMessage(`{"combinator":"and","rules":[{"field":"metadata.status","operator":"=","value":"500"}]}`),
		BucketSize: "day",
		Limit:      0,
	})
	if err != nil {
		t.Fatalf("update saved query: %v", err)
	}
	if updated.Name != "Status errors" || updated.Limit != DefaultLimit || !updated.CreatedAt.Equal(created.CreatedAt) || !updated.UpdatedAt.Equal(later) {
		t.Fatalf("updated saved query = %#v", updated)
	}

	if err := service.Delete(context.Background(), DeleteCommand{ID: created.ID, UserID: "user-1"}); err != nil {
		t.Fatalf("delete saved query: %v", err)
	}
	list, err = service.List(context.Background(), ListQuery{UserID: "user-1"})
	if err != nil {
		t.Fatalf("list after delete: %v", err)
	}
	if len(list.Items) != 0 {
		t.Fatalf("saved query list after delete = %#v", list)
	}
}

func TestServiceRejectsInvalidSavedQuery(t *testing.T) {
	service := NewService(newFakeRepository())

	_, err := service.Create(context.Background(), SaveCommand{
		UserID:     "user-1",
		Name:       "Broken",
		Query:      json.RawMessage(`[]`),
		BucketSize: "minute",
	})
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("create invalid saved query error = %v, want ErrInvalidInput", err)
	}
}

type fakeRepository struct {
	queries map[string]SavedQuery
}

func newFakeRepository() *fakeRepository {
	return &fakeRepository{queries: map[string]SavedQuery{}}
}

func (r *fakeRepository) Save(_ context.Context, query SavedQuery) (SavedQuery, error) {
	r.queries[query.ID] = query
	return query, nil
}

func (r *fakeRepository) Find(_ context.Context, query FindQuery) ([]SavedQuery, error) {
	if query.ID != "" {
		saved, ok := r.queries[query.ID]
		if !ok || saved.UserID != query.UserID {
			return []SavedQuery{}, nil
		}
		return []SavedQuery{saved}, nil
	}

	results := []SavedQuery{}
	for _, saved := range r.queries {
		if saved.UserID == query.UserID {
			results = append(results, saved)
		}
	}
	return results, nil
}

func (r *fakeRepository) Delete(_ context.Context, userID string, id string) error {
	saved, ok := r.queries[id]
	if !ok || saved.UserID != userID {
		return domain.ErrNotFound
	}
	delete(r.queries, id)
	return nil
}
