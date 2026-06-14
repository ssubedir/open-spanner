package http_test

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	appauth "github.com/ssubedir/open-spanner/internal/auth"
	"github.com/ssubedir/open-spanner/internal/config"
	httpauth "github.com/ssubedir/open-spanner/internal/metering/adapters/http/auth"
	httpmeter "github.com/ssubedir/open-spanner/internal/metering/adapters/http/meter"
	httpsubject "github.com/ssubedir/open-spanner/internal/metering/adapters/http/subject"
	httpsystem "github.com/ssubedir/open-spanner/internal/metering/adapters/http/system"
	httpusage "github.com/ssubedir/open-spanner/internal/metering/adapters/http/usage"
	"github.com/ssubedir/open-spanner/internal/metering/adapters/sqlite"
	appmeter "github.com/ssubedir/open-spanner/internal/metering/app/meter"
	appsubject "github.com/ssubedir/open-spanner/internal/metering/app/subject"
	appsystem "github.com/ssubedir/open-spanner/internal/metering/app/system"
	appusage "github.com/ssubedir/open-spanner/internal/metering/app/usage"
)

func TestAuthAPIContract(t *testing.T) {
	router := newTestRouter()

	create := requestJSON(t, router, http.MethodPost, "/v1/auth/users", map[string]any{
		"email":    " Admin@Example.COM ",
		"password": "strong-password",
	})
	if create.Code != http.StatusCreated {
		t.Fatalf("create user status = %d, want %d: %s", create.Code, http.StatusCreated, create.Body.String())
	}

	var created authUserResponse
	decodeJSON(t, create, &created)
	if created.ID == "" || created.Email != "admin@example.com" || created.CreatedAt == "" {
		t.Fatalf("created user = %#v", created)
	}

	other := requestJSON(t, router, http.MethodPost, "/v1/auth/users", map[string]any{
		"email":    "other@example.com",
		"password": "another-password",
	})
	if other.Code != http.StatusCreated {
		t.Fatalf("create second user status = %d, want %d: %s", other.Code, http.StatusCreated, other.Body.String())
	}

	duplicate := requestJSON(t, router, http.MethodPost, "/v1/auth/users", map[string]any{
		"email":    "admin@example.com",
		"password": "another-password",
	})
	if duplicate.Code != http.StatusConflict {
		t.Fatalf("duplicate user status = %d, want %d: %s", duplicate.Code, http.StatusConflict, duplicate.Body.String())
	}

	login := requestJSON(t, router, http.MethodPost, "/v1/auth/sessions", map[string]any{
		"email":    "admin@example.com",
		"password": "strong-password",
	})
	if login.Code != http.StatusCreated {
		t.Fatalf("login status = %d, want %d: %s", login.Code, http.StatusCreated, login.Body.String())
	}

	var session authSessionResponse
	decodeJSON(t, login, &session)
	if session.ExpiresAt == "" || session.User.ID != created.ID {
		t.Fatalf("session = %#v", session)
	}
	if strings.Contains(login.Body.String(), "strong-password") || strings.Contains(login.Body.String(), "token") {
		t.Fatalf("login response exposed credential material: %s", login.Body.String())
	}

	cookies := login.Result().Cookies()
	if len(cookies) != 2 {
		t.Fatalf("login cookies = %#v, want access and refresh cookies", cookies)
	}
	accessCookie := findCookie(cookies, "open_spanner_access")
	refreshCookie := findCookie(cookies, "open_spanner_refresh")
	if accessCookie == nil || accessCookie.Value == "" || !accessCookie.HttpOnly || accessCookie.SameSite != http.SameSiteLaxMode {
		t.Fatalf("access cookie = %#v", accessCookie)
	}
	if refreshCookie == nil || refreshCookie.Value == "" || !refreshCookie.HttpOnly || refreshCookie.SameSite != http.SameSiteLaxMode {
		t.Fatalf("refresh cookie = %#v", refreshCookie)
	}

	current := requestJSONWithCookies(t, router, http.MethodGet, "/v1/auth/session", nil, cookies)
	if current.Code != http.StatusOK {
		t.Fatalf("current session status = %d, want %d: %s", current.Code, http.StatusOK, current.Body.String())
	}
	var currentSession authCurrentSessionResponse
	decodeJSON(t, current, &currentSession)
	if currentSession.User.ID != created.ID {
		t.Fatalf("current session = %#v, want user %s", currentSession, created.ID)
	}

	refresh := requestJSONWithCookies(t, router, http.MethodPost, "/v1/auth/session/refresh", nil, []*http.Cookie{refreshCookie})
	if refresh.Code != http.StatusOK {
		t.Fatalf("refresh status = %d, want %d: %s", refresh.Code, http.StatusOK, refresh.Body.String())
	}
	var refreshSession authSessionResponse
	decodeJSON(t, refresh, &refreshSession)
	if refreshSession.ExpiresAt == "" || refreshSession.User.ID != created.ID {
		t.Fatalf("refresh session = %#v", refreshSession)
	}
	refreshedCookies := refresh.Result().Cookies()
	refreshedAccessCookie := findCookie(refreshedCookies, "open_spanner_access")
	refreshedRefreshCookie := findCookie(refreshedCookies, "open_spanner_refresh")
	if refreshedAccessCookie == nil || refreshedAccessCookie.Value == accessCookie.Value {
		t.Fatalf("refreshed access cookie = %#v, original = %#v", refreshedAccessCookie, accessCookie)
	}
	if refreshedRefreshCookie == nil || refreshedRefreshCookie.Value == refreshCookie.Value {
		t.Fatalf("refreshed refresh cookie = %#v, original = %#v", refreshedRefreshCookie, refreshCookie)
	}
	reusedRefresh := requestJSONWithCookies(t, router, http.MethodPost, "/v1/auth/session/refresh", nil, []*http.Cookie{refreshCookie})
	if reusedRefresh.Code != http.StatusUnauthorized {
		t.Fatalf("reused refresh status = %d, want %d: %s", reusedRefresh.Code, http.StatusUnauthorized, reusedRefresh.Body.String())
	}

	logoutCookies := []*http.Cookie{refreshedAccessCookie, refreshedRefreshCookie}
	logout := requestJSONWithCookies(t, router, http.MethodDelete, "/v1/auth/session", nil, logoutCookies)
	if logout.Code != http.StatusNoContent {
		t.Fatalf("logout status = %d, want %d: %s", logout.Code, http.StatusNoContent, logout.Body.String())
	}
	cleared := logout.Result().Cookies()
	clearedAccess := findCookie(cleared, "open_spanner_access")
	clearedRefresh := findCookie(cleared, "open_spanner_refresh")
	if len(cleared) != 2 || clearedAccess == nil || clearedAccess.MaxAge != -1 || clearedRefresh == nil || clearedRefresh.MaxAge != -1 {
		t.Fatalf("logout cookies = %#v, want cleared auth cookies", cleared)
	}

	deleted := requestJSONWithCookies(t, router, http.MethodGet, "/v1/auth/session", nil, logoutCookies)
	if deleted.Code != http.StatusUnauthorized {
		t.Fatalf("deleted session status = %d, want %d: %s", deleted.Code, http.StatusUnauthorized, deleted.Body.String())
	}

	badLogin := requestJSON(t, router, http.MethodPost, "/v1/auth/sessions", map[string]any{
		"email":    "admin@example.com",
		"password": "wrong-password",
	})
	if badLogin.Code != http.StatusUnauthorized {
		t.Fatalf("bad login status = %d, want %d: %s", badLogin.Code, http.StatusUnauthorized, badLogin.Body.String())
	}
}

func TestMeterAPIContract(t *testing.T) {
	router := newTestRouter()

	create := requestJSON(t, router, http.MethodPost, "/v1/meters", map[string]any{
		"name":                 "api_calls",
		"description":          "API calls",
		"unit":                 "call",
		"aggregation":          "sum",
		"event_retention_days": 30,
	})
	if create.Code != http.StatusCreated {
		t.Fatalf("create meter status = %d, want %d: %s", create.Code, http.StatusCreated, create.Body.String())
	}

	var created meterResponse
	decodeJSON(t, create, &created)
	if created.ID == "" || created.Name != "api_calls" || created.EventRetentionDays != 30 {
		t.Fatalf("created meter = %#v", created)
	}

	list := requestJSON(t, router, http.MethodGet, "/v1/meters", nil)
	if list.Code != http.StatusOK {
		t.Fatalf("list meters status = %d, want %d: %s", list.Code, http.StatusOK, list.Body.String())
	}

	var listed meterListResponse
	decodeJSON(t, list, &listed)
	if len(listed.Items) != 1 || listed.Items[0].ID != created.ID {
		t.Fatalf("listed meters = %#v", listed)
	}

	get := requestJSON(t, router, http.MethodGet, "/v1/meters/"+created.ID, nil)
	if get.Code != http.StatusOK {
		t.Fatalf("get meter status = %d, want %d: %s", get.Code, http.StatusOK, get.Body.String())
	}

	var fetched meterResponse
	decodeJSON(t, get, &fetched)
	if fetched.ID != created.ID || fetched.Name != created.Name {
		t.Fatalf("fetched meter = %#v, created = %#v", fetched, created)
	}

	update := requestJSON(t, router, http.MethodPut, "/v1/meters/"+created.ID, map[string]any{
		"description": "Updated API calls",
	})
	if update.Code != http.StatusOK {
		t.Fatalf("update meter status = %d, want %d: %s", update.Code, http.StatusOK, update.Body.String())
	}
	var updated meterResponse
	decodeJSON(t, update, &updated)
	if updated.Description != "Updated API calls" || updated.Name != created.Name {
		t.Fatalf("updated meter = %#v", updated)
	}

	del := requestJSON(t, router, http.MethodDelete, "/v1/meters/"+created.ID, nil)
	if del.Code != http.StatusNoContent {
		t.Fatalf("delete meter status = %d, want %d: %s", del.Code, http.StatusNoContent, del.Body.String())
	}

	getDeleted := requestJSON(t, router, http.MethodGet, "/v1/meters/"+created.ID, nil)
	if getDeleted.Code != http.StatusNotFound {
		t.Fatalf("get deleted meter status = %d, want %d: %s", getDeleted.Code, http.StatusNotFound, getDeleted.Body.String())
	}
}

func TestMeterAPIListLimit(t *testing.T) {
	router := newTestRouter()

	for _, name := range []string{"api_calls", "tokens"} {
		create := requestJSON(t, router, http.MethodPost, "/v1/meters", map[string]any{
			"name":        name,
			"description": name,
			"unit":        "count",
			"aggregation": "sum",
		})
		if create.Code != http.StatusCreated {
			t.Fatalf("create meter status = %d: %s", create.Code, create.Body.String())
		}
	}

	list := requestJSON(t, router, http.MethodGet, "/v1/meters?limit=1", nil)
	if list.Code != http.StatusOK {
		t.Fatalf("list meters status = %d, want %d: %s", list.Code, http.StatusOK, list.Body.String())
	}

	var meters meterListResponse
	decodeJSON(t, list, &meters)
	if len(meters.Items) != 1 || meters.NextCursor == "" {
		t.Fatalf("meter page = %#v, want one item with next cursor", meters)
	}

	next := requestJSON(t, router, http.MethodGet, "/v1/meters?limit=1&cursor="+meters.NextCursor, nil)
	if next.Code != http.StatusOK {
		t.Fatalf("next meters status = %d, want %d: %s", next.Code, http.StatusOK, next.Body.String())
	}
	var nextMeters meterListResponse
	decodeJSON(t, next, &nextMeters)
	if len(nextMeters.Items) != 1 || nextMeters.NextCursor != "" {
		t.Fatalf("next meter page = %#v", nextMeters)
	}
}

func TestMeterStatsAPIContract(t *testing.T) {
	router := newTestRouter()

	for _, name := range []string{"api_calls", "tokens"} {
		createMeter := requestJSON(t, router, http.MethodPost, "/v1/meters", map[string]any{
			"name":                 name,
			"description":          name,
			"unit":                 "event",
			"aggregation":          "sum",
			"event_retention_days": 30,
		})
		if createMeter.Code != http.StatusCreated {
			t.Fatalf("create meter status = %d: %s", createMeter.Code, createMeter.Body.String())
		}
	}

	for _, event := range []map[string]any{
		{"subject": "org_123", "meter": "api_calls", "quantity": 1, "timestamp": "2026-06-08T10:00:00Z"},
		{"subject": "org_123", "meter": "api_calls", "quantity": 1, "timestamp": "2026-06-08T12:00:00Z"},
	} {
		createUsage := requestJSON(t, router, http.MethodPost, "/v1/usages", event)
		if createUsage.Code != http.StatusCreated {
			t.Fatalf("create usage status = %d: %s", createUsage.Code, createUsage.Body.String())
		}
	}

	stats := requestJSON(t, router, http.MethodGet, "/v1/meters/stats", nil)
	if stats.Code != http.StatusOK {
		t.Fatalf("meter stats status = %d, want %d: %s", stats.Code, http.StatusOK, stats.Body.String())
	}

	var result meterStatsListResponse
	decodeJSON(t, stats, &result)
	if len(result.Items) != 2 {
		t.Fatalf("meter stats = %#v", result)
	}

	byMeter := map[string]meterStatsResponse{}
	for _, stat := range result.Items {
		byMeter[stat.Meter] = stat
	}
	if byMeter["api_calls"].UsageEvents != 2 || byMeter["api_calls"].LastEventAt != "2026-06-08T12:00:00Z" || byMeter["api_calls"].EventRetentionDays != 30 {
		t.Fatalf("api_calls stats = %#v", byMeter["api_calls"])
	}
	if byMeter["tokens"].UsageEvents != 0 || byMeter["tokens"].LastEventAt != "" || byMeter["tokens"].EventRetentionDays != 30 {
		t.Fatalf("tokens stats = %#v", byMeter["tokens"])
	}
}

func TestSubjectStatsAPIContract(t *testing.T) {
	router := newTestRouter()

	for _, name := range []string{"api_calls", "tokens"} {
		createMeter := requestJSON(t, router, http.MethodPost, "/v1/meters", map[string]any{
			"name":        name,
			"description": name,
			"unit":        "event",
			"aggregation": "sum",
		})
		if createMeter.Code != http.StatusCreated {
			t.Fatalf("create meter status = %d: %s", createMeter.Code, createMeter.Body.String())
		}
	}

	for _, event := range []map[string]any{
		{"subject": "org_123", "meter": "api_calls", "quantity": 1, "timestamp": "2026-06-08T10:00:00Z"},
		{"subject": "org_123", "meter": "tokens", "quantity": 1, "timestamp": "2026-06-08T11:00:00Z"},
		{"subject": "org_456", "meter": "api_calls", "quantity": 1, "timestamp": "2026-06-08T12:00:00Z"},
	} {
		createUsage := requestJSON(t, router, http.MethodPost, "/v1/usages", event)
		if createUsage.Code != http.StatusCreated {
			t.Fatalf("create usage status = %d: %s", createUsage.Code, createUsage.Body.String())
		}
	}

	stats := requestJSON(t, router, http.MethodGet, "/v1/subjects?limit=10", nil)
	if stats.Code != http.StatusOK {
		t.Fatalf("subject stats status = %d, want %d: %s", stats.Code, http.StatusOK, stats.Body.String())
	}

	var result subjectStatsListResponse
	decodeJSON(t, stats, &result)
	if len(result.Items) != 2 {
		t.Fatalf("subject stats = %#v", result)
	}
	if result.Items[0].Subject != "org_456" || result.Items[0].UsageEvents != 1 || result.Items[0].Meters != 1 || result.Items[0].LastEventAt != "2026-06-08T12:00:00Z" {
		t.Fatalf("first subject stats = %#v", result.Items[0])
	}
	if result.Items[1].Subject != "org_123" || result.Items[1].UsageEvents != 2 || result.Items[1].Meters != 2 || result.Items[1].LastEventAt != "2026-06-08T11:00:00Z" {
		t.Fatalf("second subject stats = %#v", result.Items[1])
	}
}

func TestSubjectUsageEventsAPIContract(t *testing.T) {
	router := newTestRouter()

	createMeter := requestJSON(t, router, http.MethodPost, "/v1/meters", map[string]any{
		"name":        "api_calls",
		"description": "API calls",
		"unit":        "event",
		"aggregation": "sum",
	})
	if createMeter.Code != http.StatusCreated {
		t.Fatalf("create meter status = %d: %s", createMeter.Code, createMeter.Body.String())
	}

	for _, event := range []map[string]any{
		{"idempotency_key": "subject-events-1", "subject": "org_123", "meter": "api_calls", "quantity": 1, "timestamp": "2026-06-08T10:00:00Z"},
		{"idempotency_key": "subject-events-2", "subject": "org_456", "meter": "api_calls", "quantity": 9, "timestamp": "2026-06-08T11:00:00Z"},
		{"idempotency_key": "subject-events-3", "subject": "org_123", "meter": "api_calls", "quantity": 2, "timestamp": "2026-06-08T12:00:00Z"},
	} {
		createUsage := requestJSON(t, router, http.MethodPost, "/v1/usages", event)
		if createUsage.Code != http.StatusCreated {
			t.Fatalf("create usage status = %d: %s", createUsage.Code, createUsage.Body.String())
		}
	}

	list := requestJSON(t, router, http.MethodGet, "/v1/subjects/org_123/usageevents?limit=10", nil)
	if list.Code != http.StatusOK {
		t.Fatalf("subject usage events status = %d, want %d: %s", list.Code, http.StatusOK, list.Body.String())
	}

	var events []usageResponse
	decodeJSON(t, list, &events)
	if len(events) != 2 {
		t.Fatalf("subject usage events = %#v", events)
	}
	if events[0].Subject != "org_123" || events[0].Quantity != 2 || events[0].Timestamp != "2026-06-08T12:00:00Z" {
		t.Fatalf("first subject usage event = %#v", events[0])
	}
	if events[1].Subject != "org_123" || events[1].Quantity != 1 || events[1].Timestamp != "2026-06-08T10:00:00Z" {
		t.Fatalf("second subject usage event = %#v", events[1])
	}
}

func TestUsageAPIContract(t *testing.T) {
	router := newTestRouter()

	createMeter := requestJSON(t, router, http.MethodPost, "/v1/meters", map[string]any{
		"name":            "tokens",
		"description":     "Tokens",
		"unit":            "token",
		"aggregation":     "sum",
		"metadata_schema": map[string]string{"region": "string"},
	})
	if createMeter.Code != http.StatusCreated {
		t.Fatalf("create meter status = %d: %s", createMeter.Code, createMeter.Body.String())
	}

	createUsage := requestJSON(t, router, http.MethodPost, "/v1/usages", map[string]any{
		"idempotency_key": "usage-1",
		"subject":         "org_123",
		"meter":           "tokens",
		"quantity":        11,
		"timestamp":       "2026-06-08T14:30:00Z",
		"metadata": map[string]any{
			"region": "us-east-1",
		},
	})
	if createUsage.Code != http.StatusCreated {
		t.Fatalf("create usage status = %d, want %d: %s", createUsage.Code, http.StatusCreated, createUsage.Body.String())
	}

	var usage usageResponse
	decodeJSON(t, createUsage, &usage)
	if usage.ID == "" || usage.Subject != "org_123" || usage.Meter != "tokens" || usage.Quantity != 11 {
		t.Fatalf("created usage = %#v", usage)
	}

	otherUsage := requestJSON(t, router, http.MethodPost, "/v1/usages", map[string]any{
		"idempotency_key": "usage-2",
		"subject":         "org_123",
		"meter":           "tokens",
		"quantity":        5,
		"timestamp":       "2026-06-08T15:30:00Z",
		"metadata": map[string]any{
			"region": "us-west-2",
		},
	})
	if otherUsage.Code != http.StatusCreated {
		t.Fatalf("create other usage status = %d, want %d: %s", otherUsage.Code, http.StatusCreated, otherUsage.Body.String())
	}

	list := requestJSON(t, router, http.MethodGet, "/v1/usages?subject=org_123&meter=tokens&from=2026-06-08T00:00:00Z&to=2026-06-09T00:00:00Z&bucket_size=day", nil)
	if list.Code != http.StatusOK {
		t.Fatalf("list usages status = %d, want %d: %s", list.Code, http.StatusOK, list.Body.String())
	}

	var buckets []usageListItemResponse
	decodeJSON(t, list, &buckets)
	if len(buckets) != 1 {
		t.Fatalf("bucket count = %d, want 1", len(buckets))
	}
	if buckets[0].Quantity != 16 || buckets[0].BucketStart != "2026-06-08T00:00:00Z" {
		t.Fatalf("bucket = %#v", buckets[0])
	}
	if buckets[0].Aggregation != "sum" || buckets[0].Unit != "token" {
		t.Fatalf("bucket context = %#v", buckets[0])
	}

	eventList := requestJSON(t, router, http.MethodGet, "/v1/usageevents?subject=org_123&meter=tokens&from=2026-06-08T00:00:00Z&to=2026-06-09T00:00:00Z&limit=1", nil)
	if eventList.Code != http.StatusOK {
		t.Fatalf("list usage events status = %d, want %d: %s", eventList.Code, http.StatusOK, eventList.Body.String())
	}

	var events eventListResponse
	decodeJSON(t, eventList, &events)
	if len(events.Items) != 1 || events.Items[0].Quantity != 5 || events.Items[0].Timestamp != "2026-06-08T15:30:00Z" || events.NextCursor == "" {
		t.Fatalf("usage events = %#v", events)
	}

	nextEventList := requestJSON(t, router, http.MethodGet, "/v1/usageevents?subject=org_123&meter=tokens&from=2026-06-08T00:00:00Z&to=2026-06-09T00:00:00Z&limit=1&cursor="+events.NextCursor, nil)
	if nextEventList.Code != http.StatusOK {
		t.Fatalf("list next usage events status = %d, want %d: %s", nextEventList.Code, http.StatusOK, nextEventList.Body.String())
	}

	var nextEvents eventListResponse
	decodeJSON(t, nextEventList, &nextEvents)
	if len(nextEvents.Items) != 1 || nextEvents.Items[0].Quantity != 11 || nextEvents.NextCursor != "" {
		t.Fatalf("next usage events = %#v", nextEvents)
	}

	filteredList := requestJSON(t, router, http.MethodGet, "/v1/usages?subject=org_123&meter=tokens&from=2026-06-08T00:00:00Z&to=2026-06-09T00:00:00Z&bucket_size=day&metadata.region=us-east-1", nil)
	if filteredList.Code != http.StatusOK {
		t.Fatalf("filtered list usages status = %d, want %d: %s", filteredList.Code, http.StatusOK, filteredList.Body.String())
	}

	var filteredBuckets []usageListItemResponse
	decodeJSON(t, filteredList, &filteredBuckets)
	if len(filteredBuckets) != 1 || filteredBuckets[0].Quantity != 11 {
		t.Fatalf("filtered buckets = %#v, want one bucket with quantity 11", filteredBuckets)
	}

	searchList := requestJSON(t, router, http.MethodPost, "/v1/usages/search", map[string]any{
		"subject":     "org_123",
		"meter":       "tokens",
		"from":        "2026-06-08T00:00:00Z",
		"to":          "2026-06-09T00:00:00Z",
		"bucket_size": "day",
		"filter": map[string]any{
			"type":  "condition",
			"field": "metadata.region",
			"op":    "eq",
			"value": "us-west-2",
		},
	})
	if searchList.Code != http.StatusOK {
		t.Fatalf("search usages status = %d, want %d: %s", searchList.Code, http.StatusOK, searchList.Body.String())
	}

	var searchBuckets []usageListItemResponse
	decodeJSON(t, searchList, &searchBuckets)
	if len(searchBuckets) != 1 || searchBuckets[0].Quantity != 5 {
		t.Fatalf("search buckets = %#v, want one bucket with quantity 5", searchBuckets)
	}

	eventSearch := requestJSON(t, router, http.MethodPost, "/v1/usageevents/search", map[string]any{
		"subject": "org_123",
		"meter":   "tokens",
		"from":    "2026-06-08T00:00:00Z",
		"to":      "2026-06-09T00:00:00Z",
		"limit":   10,
		"filter": map[string]any{
			"type":  "condition",
			"field": "quantity",
			"op":    "gte",
			"value": 10,
		},
	})
	if eventSearch.Code != http.StatusOK {
		t.Fatalf("search usage events status = %d, want %d: %s", eventSearch.Code, http.StatusOK, eventSearch.Body.String())
	}

	var searchedEvents eventListResponse
	decodeJSON(t, eventSearch, &searchedEvents)
	if len(searchedEvents.Items) != 1 || searchedEvents.Items[0].Quantity != 11 {
		t.Fatalf("searched usage events = %#v, want one event with quantity 11", searchedEvents)
	}
}

func TestUsageEventPruneAPIContract(t *testing.T) {
	router := newTestRouter()

	createMeter := requestJSON(t, router, http.MethodPost, "/v1/meters", map[string]any{
		"name":                 "retained_events",
		"description":          "Retained events",
		"unit":                 "event",
		"aggregation":          "sum",
		"event_retention_days": 1,
	})
	if createMeter.Code != http.StatusCreated {
		t.Fatalf("create meter status = %d: %s", createMeter.Code, createMeter.Body.String())
	}

	for _, event := range []map[string]any{
		{"subject": "org_123", "meter": "retained_events", "quantity": 1, "timestamp": "2026-01-01T00:00:00Z"},
		{"subject": "org_123", "meter": "retained_events", "quantity": 2, "timestamp": time.Now().UTC().Format(time.RFC3339)},
	} {
		createUsage := requestJSON(t, router, http.MethodPost, "/v1/usages", event)
		if createUsage.Code != http.StatusCreated {
			t.Fatalf("create usage status = %d: %s", createUsage.Code, createUsage.Body.String())
		}
	}

	dryRun := requestJSON(t, router, http.MethodPost, "/v1/usageevents/prune?dry_run=true", nil)
	if dryRun.Code != http.StatusOK {
		t.Fatalf("dry-run prune status = %d, want %d: %s", dryRun.Code, http.StatusOK, dryRun.Body.String())
	}

	var dryRunResult pruneResponse
	decodeJSON(t, dryRun, &dryRunResult)
	if dryRunResult.Deleted != 1 || !dryRunResult.DryRun {
		t.Fatalf("dry-run prune result = %#v", dryRunResult)
	}

	dryRunList := requestJSON(t, router, http.MethodGet, "/v1/usageevents?subject=org_123&meter=retained_events&limit=10", nil)
	if dryRunList.Code != http.StatusOK {
		t.Fatalf("dry-run list usage events status = %d: %s", dryRunList.Code, dryRunList.Body.String())
	}
	var dryRunEvents eventListResponse
	decodeJSON(t, dryRunList, &dryRunEvents)
	if len(dryRunEvents.Items) != 2 {
		t.Fatalf("dry-run deleted events = %#v", dryRunEvents)
	}

	prune := requestJSON(t, router, http.MethodPost, "/v1/usageevents/prune", nil)
	if prune.Code != http.StatusOK {
		t.Fatalf("prune status = %d, want %d: %s", prune.Code, http.StatusOK, prune.Body.String())
	}

	var result pruneResponse
	decodeJSON(t, prune, &result)
	if result.Deleted != 1 || result.DryRun || len(result.Meters) != 1 || result.Meters[0].Meter != "retained_events" || result.Meters[0].Deleted != 1 || result.Meters[0].Before == "" {
		t.Fatalf("prune result = %#v", result)
	}

	history := requestJSON(t, router, http.MethodGet, "/v1/usageevents/prunes?limit=10", nil)
	if history.Code != http.StatusOK {
		t.Fatalf("prune history status = %d, want %d: %s", history.Code, http.StatusOK, history.Body.String())
	}

	var runs pruneListResponse
	decodeJSON(t, history, &runs)
	if len(runs.Items) != 2 || pruneResponseCountByMode(runs.Items, true) != 1 || pruneResponseCountByMode(runs.Items, false) != 1 {
		t.Fatalf("prune history = %#v", runs)
	}
	for _, run := range runs.Items {
		if run.ID == "" || run.CreatedAt == "" || len(run.Meters) != 1 {
			t.Fatalf("prune run missing fields = %#v", run)
		}
	}

	list := requestJSON(t, router, http.MethodGet, "/v1/usageevents?subject=org_123&meter=retained_events&limit=10", nil)
	if list.Code != http.StatusOK {
		t.Fatalf("list usage events status = %d: %s", list.Code, list.Body.String())
	}

	var events eventListResponse
	decodeJSON(t, list, &events)
	if len(events.Items) != 1 || events.Items[0].Quantity != 2 {
		t.Fatalf("remaining events = %#v", events)
	}
}

func pruneResponseCountByMode(runs []pruneResponse, dryRun bool) int {
	count := 0
	for _, run := range runs {
		if run.DryRun == dryRun {
			count++
		}
	}
	return count
}

func TestUsageBulkAPIContract(t *testing.T) {
	router := newTestRouter()

	createMeter := requestJSON(t, router, http.MethodPost, "/v1/meters", map[string]any{
		"name":            "api_calls",
		"description":     "API calls",
		"unit":            "call",
		"aggregation":     "sum",
		"metadata_schema": map[string]string{"region": "string"},
	})
	if createMeter.Code != http.StatusCreated {
		t.Fatalf("create meter status = %d: %s", createMeter.Code, createMeter.Body.String())
	}

	bulkPayload := []map[string]any{
		{
			"idempotency_key": "bulk-1",
			"subject":         "org_123",
			"meter":           "api_calls",
			"quantity":        2,
			"timestamp":       "2026-06-08T10:00:00Z",
			"metadata": map[string]any{
				"region": "us-east-1",
			},
		},
		{
			"idempotency_key": "bulk-2",
			"subject":         "org_123",
			"meter":           "api_calls",
			"quantity":        3,
			"timestamp":       "2026-06-08T11:00:00Z",
			"metadata": map[string]any{
				"region": "us-east-1",
			},
		},
	}
	createBulk := requestJSONWithHeaders(t, router, http.MethodPost, "/v1/usages/bulk", bulkPayload, map[string]string{
		"Idempotency-Key": "batch-1",
	})
	if createBulk.Code != http.StatusCreated {
		t.Fatalf("create bulk status = %d, want %d: %s", createBulk.Code, http.StatusCreated, createBulk.Body.String())
	}

	var created bulkResponse
	decodeJSON(t, createBulk, &created)
	if created.AcceptedCount != 2 || created.DuplicateCount != 0 || created.FailedCount != 0 || len(created.Accepted) != 2 || created.Accepted[0].ID == "" || created.Accepted[1].ID == "" {
		t.Fatalf("created bulk usage = %#v", created)
	}

	replayedBulk := requestJSONWithHeaders(t, router, http.MethodPost, "/v1/usages/bulk", []map[string]any{
		{
			"idempotency_key": "bulk-3",
			"subject":         "org_123",
			"meter":           "api_calls",
			"quantity":        100,
			"timestamp":       "2026-06-08T12:00:00Z",
			"metadata": map[string]any{
				"region": "us-east-1",
			},
		},
	}, map[string]string{
		"Idempotency-Key": "batch-1",
	})
	if replayedBulk.Code != http.StatusCreated {
		t.Fatalf("replay bulk status = %d, want %d: %s", replayedBulk.Code, http.StatusCreated, replayedBulk.Body.String())
	}

	var replayed bulkResponse
	decodeJSON(t, replayedBulk, &replayed)
	if replayed.AcceptedCount != 2 || replayed.DuplicateCount != 0 || len(replayed.Accepted) != 2 || replayed.Accepted[0].ID != created.Accepted[0].ID || replayed.Accepted[1].ID != created.Accepted[1].ID {
		t.Fatalf("replayed bulk usage = %#v, want original %#v", replayed, created)
	}

	duplicateBulk := requestJSON(t, router, http.MethodPost, "/v1/usages/bulk", []map[string]any{
		{
			"idempotency_key": "bulk-1",
			"subject":         "org_123",
			"meter":           "api_calls",
			"quantity":        100,
			"timestamp":       "2026-06-08T12:00:00Z",
			"metadata": map[string]any{
				"region": "us-east-1",
			},
		},
		{
			"idempotency_key": "bulk-4",
			"subject":         "org_123",
			"meter":           "api_calls",
			"quantity":        7,
			"timestamp":       "2026-06-08T13:00:00Z",
			"metadata": map[string]any{
				"region": "us-east-1",
			},
		},
	})
	if duplicateBulk.Code != http.StatusCreated {
		t.Fatalf("duplicate bulk status = %d, want %d: %s", duplicateBulk.Code, http.StatusCreated, duplicateBulk.Body.String())
	}

	var duplicateResult bulkResponse
	decodeJSON(t, duplicateBulk, &duplicateResult)
	if duplicateResult.AcceptedCount != 1 || duplicateResult.DuplicateCount != 1 || duplicateResult.FailedCount != 0 || len(duplicateResult.Accepted) != 1 || len(duplicateResult.Duplicates) != 1 {
		t.Fatalf("duplicate bulk result = %#v", duplicateResult)
	}
	if duplicateResult.Duplicates[0].ID != created.Accepted[0].ID || duplicateResult.Duplicates[0].Quantity != 2 {
		t.Fatalf("duplicate item = %#v, want original %#v", duplicateResult.Duplicates[0], created.Accepted[0])
	}

	list := requestJSON(t, router, http.MethodGet, "/v1/usages?subject=org_123&meter=api_calls&from=2026-06-08T00:00:00Z&to=2026-06-09T00:00:00Z&bucket_size=day&metadata.region=us-east-1", nil)
	if list.Code != http.StatusOK {
		t.Fatalf("list usages status = %d, want %d: %s", list.Code, http.StatusOK, list.Body.String())
	}

	var buckets []usageListItemResponse
	decodeJSON(t, list, &buckets)
	if len(buckets) != 1 || buckets[0].Quantity != 12 {
		t.Fatalf("bulk usage buckets = %#v, want one bucket with quantity 12", buckets)
	}
}

func TestUsageBulkAPIRejectsEmptyBatch(t *testing.T) {
	router := newTestRouter()

	res := requestJSON(t, router, http.MethodPost, "/v1/usages/bulk", []map[string]any{})
	if res.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d: %s", res.Code, http.StatusBadRequest, res.Body.String())
	}

	var errRes errorResponse
	decodeJSON(t, res, &errRes)
	if errRes.Error.Code != "invalid_input" {
		t.Fatalf("error code = %q, want invalid_input", errRes.Error.Code)
	}
}

func TestUsageBulkAPIPartialSuccess(t *testing.T) {
	router := newTestRouter()

	createMeter := requestJSON(t, router, http.MethodPost, "/v1/meters", map[string]any{
		"name":            "api_calls",
		"description":     "API Calls",
		"unit":            "call",
		"aggregation":     "sum",
		"metadata_schema": map[string]string{"region": "string"},
	})
	if createMeter.Code != http.StatusCreated {
		t.Fatalf("create meter status = %d: %s", createMeter.Code, createMeter.Body.String())
	}

	res := requestJSON(t, router, http.MethodPost, "/v1/usages/bulk", []map[string]any{
		{
			"idempotency_key": "partial-1",
			"subject":         "org_123",
			"meter":           "api_calls",
			"quantity":        2,
			"timestamp":       "2026-06-08T10:00:00Z",
			"metadata": map[string]any{
				"region": "us-east-1",
			},
		},
		{
			"idempotency_key": "partial-2",
			"subject":         "org_123",
			"meter":           "api_calls",
			"quantity":        3,
			"timestamp":       "nope",
			"metadata": map[string]any{
				"region": "us-east-1",
			},
		},
		{
			"idempotency_key": "partial-3",
			"subject":         "org_123",
			"meter":           "missing",
			"quantity":        5,
			"timestamp":       "2026-06-08T11:00:00Z",
		},
	})
	if res.Code != http.StatusCreated {
		t.Fatalf("partial bulk status = %d, want %d: %s", res.Code, http.StatusCreated, res.Body.String())
	}

	var result bulkResponse
	decodeJSON(t, res, &result)
	if result.AcceptedCount != 1 || result.DuplicateCount != 0 || result.FailedCount != 2 || len(result.Accepted) != 1 || len(result.Failed) != 2 {
		t.Fatalf("partial bulk result = %#v", result)
	}
	if result.Failed[0].Index != 1 || result.Failed[0].Code != "invalid_timestamp" {
		t.Fatalf("timestamp failure = %#v", result.Failed[0])
	}
	if result.Failed[1].Index != 2 || result.Failed[1].Code != "not_found" {
		t.Fatalf("missing meter failure = %#v", result.Failed[1])
	}

	list := requestJSON(t, router, http.MethodGet, "/v1/usages?subject=org_123&meter=api_calls&from=2026-06-08T00:00:00Z&to=2026-06-09T00:00:00Z&bucket_size=day", nil)
	if list.Code != http.StatusOK {
		t.Fatalf("list usages status = %d: %s", list.Code, list.Body.String())
	}
	var buckets []usageListItemResponse
	decodeJSON(t, list, &buckets)
	if len(buckets) != 1 || buckets[0].Quantity != 2 {
		t.Fatalf("bulk usage buckets = %#v, want quantity 2", buckets)
	}
}

func TestUsageBulkAPIReturnsBadRequestWhenAllItemsFail(t *testing.T) {
	router := newTestRouter()

	res := requestJSON(t, router, http.MethodPost, "/v1/usages/bulk", []map[string]any{
		{
			"subject":   "org_123",
			"meter":     "missing",
			"quantity":  1,
			"timestamp": "2026-06-08T10:00:00Z",
		},
		{
			"subject":   "org_123",
			"meter":     "missing",
			"quantity":  1,
			"timestamp": "bad",
		},
	})
	if res.Code != http.StatusBadRequest {
		t.Fatalf("failed bulk status = %d, want %d: %s", res.Code, http.StatusBadRequest, res.Body.String())
	}

	var result bulkResponse
	decodeJSON(t, res, &result)
	if result.AcceptedCount != 0 || result.DuplicateCount != 0 || result.FailedCount != 2 || len(result.Failed) != 2 {
		t.Fatalf("failed bulk result = %#v", result)
	}
}

func TestUsageAPIListLimit(t *testing.T) {
	router := newTestRouter()

	createMeter := requestJSON(t, router, http.MethodPost, "/v1/meters", map[string]any{
		"name":        "api_calls",
		"description": "API calls",
		"unit":        "call",
		"aggregation": "sum",
	})
	if createMeter.Code != http.StatusCreated {
		t.Fatalf("create meter status = %d: %s", createMeter.Code, createMeter.Body.String())
	}

	for day := 1; day <= 2; day++ {
		createUsage := requestJSON(t, router, http.MethodPost, "/v1/usages", map[string]any{
			"subject":   "org_123",
			"meter":     "api_calls",
			"quantity":  1,
			"timestamp": "2026-06-0" + string(rune('0'+day)) + "T10:00:00Z",
		})
		if createUsage.Code != http.StatusCreated {
			t.Fatalf("create usage status = %d: %s", createUsage.Code, createUsage.Body.String())
		}
	}

	list := requestJSON(t, router, http.MethodGet, "/v1/usages?subject=org_123&meter=api_calls&from=2026-06-01T00:00:00Z&to=2026-06-03T00:00:00Z&bucket_size=day&limit=1", nil)
	if list.Code != http.StatusOK {
		t.Fatalf("list usages status = %d, want %d: %s", list.Code, http.StatusOK, list.Body.String())
	}

	var buckets []usageListItemResponse
	decodeJSON(t, list, &buckets)
	if len(buckets) != 1 {
		t.Fatalf("bucket count = %d, want 1", len(buckets))
	}
}

func TestUsageExportAPIContract(t *testing.T) {
	router := newTestRouter()

	createMeter := requestJSON(t, router, http.MethodPost, "/v1/meters", map[string]any{
		"name":        "api_calls",
		"description": "API calls",
		"unit":        "call",
		"aggregation": "sum",
	})
	if createMeter.Code != http.StatusCreated {
		t.Fatalf("create meter status = %d: %s", createMeter.Code, createMeter.Body.String())
	}

	createUsage := requestJSON(t, router, http.MethodPost, "/v1/usages", map[string]any{
		"subject":   "org_123",
		"meter":     "api_calls",
		"quantity":  7,
		"timestamp": "2026-06-08T10:00:00Z",
	})
	if createUsage.Code != http.StatusCreated {
		t.Fatalf("create usage status = %d: %s", createUsage.Code, createUsage.Body.String())
	}

	export := requestJSON(t, router, http.MethodGet, "/v1/usages/export?subject=org_123&meter=api_calls&from=2026-06-08T00:00:00Z&to=2026-06-09T00:00:00Z&bucket_size=day", nil)
	if export.Code != http.StatusOK {
		t.Fatalf("export status = %d, want %d: %s", export.Code, http.StatusOK, export.Body.String())
	}
	if contentType := export.Header().Get("Content-Type"); !strings.HasPrefix(contentType, "text/csv") {
		t.Fatalf("content type = %q, want text/csv", contentType)
	}

	want := "bucket_start,subject,meter,bucket_size,aggregation,unit,quantity\n2026-06-08T00:00:00Z,org_123,api_calls,day,sum,call,7\n"
	if export.Body.String() != want {
		t.Fatalf("csv = %q, want %q", export.Body.String(), want)
	}
}

func TestUsageEventExportAPIContract(t *testing.T) {
	router := newTestRouter()

	createMeter := requestJSON(t, router, http.MethodPost, "/v1/meters", map[string]any{
		"name":            "api_calls",
		"description":     "API calls",
		"unit":            "call",
		"aggregation":     "sum",
		"metadata_schema": map[string]string{"region": "string"},
	})
	if createMeter.Code != http.StatusCreated {
		t.Fatalf("create meter status = %d: %s", createMeter.Code, createMeter.Body.String())
	}

	createUsage := requestJSON(t, router, http.MethodPost, "/v1/usages", map[string]any{
		"idempotency_key": "usage-event-export-1",
		"subject":         "org_123",
		"meter":           "api_calls",
		"quantity":        7,
		"timestamp":       "2026-06-08T10:00:00Z",
		"metadata":        map[string]any{"region": "us-east-1"},
	})
	if createUsage.Code != http.StatusCreated {
		t.Fatalf("create usage status = %d: %s", createUsage.Code, createUsage.Body.String())
	}

	export := requestJSON(t, router, http.MethodGet, "/v1/usageevents/export?subject=org_123&meter=api_calls&from=2026-06-08T00:00:00Z&to=2026-06-09T00:00:00Z&limit=100", nil)
	if export.Code != http.StatusOK {
		t.Fatalf("event export status = %d, want %d: %s", export.Code, http.StatusOK, export.Body.String())
	}
	if contentType := export.Header().Get("Content-Type"); !strings.HasPrefix(contentType, "text/csv") {
		t.Fatalf("content type = %q, want text/csv", contentType)
	}

	records, err := csv.NewReader(bytes.NewReader(export.Body.Bytes())).ReadAll()
	if err != nil {
		t.Fatalf("read event csv: %v", err)
	}
	if len(records) != 2 {
		t.Fatalf("csv records = %#v", records)
	}
	wantHeader := []string{"timestamp", "received_at", "subject", "meter", "quantity", "metadata", "id", "idempotency_key"}
	if strings.Join(records[0], ",") != strings.Join(wantHeader, ",") {
		t.Fatalf("csv header = %#v, want %#v", records[0], wantHeader)
	}
	if records[1][0] != "2026-06-08T10:00:00Z" || records[1][2] != "org_123" || records[1][3] != "api_calls" || records[1][4] != "7" || records[1][5] != `{"region":"us-east-1"}` || records[1][7] != "usage-event-export-1" {
		t.Fatalf("csv event row = %#v", records[1])
	}
	if records[1][1] == "" || records[1][6] == "" {
		t.Fatalf("csv generated fields missing: %#v", records[1])
	}
}

func TestUsageAPIGroupByMetadata(t *testing.T) {
	router := newTestRouter()

	createMeter := requestJSON(t, router, http.MethodPost, "/v1/meters", map[string]any{
		"name":            "api_calls",
		"description":     "API calls",
		"unit":            "call",
		"aggregation":     "sum",
		"metadata_schema": map[string]string{"region": "string"},
	})
	if createMeter.Code != http.StatusCreated {
		t.Fatalf("create meter status = %d: %s", createMeter.Code, createMeter.Body.String())
	}

	events := []map[string]any{
		{"subject": "org_123", "meter": "api_calls", "quantity": 2, "timestamp": "2026-06-08T10:00:00Z", "metadata": map[string]any{"region": "us-east-1"}},
		{"subject": "org_123", "meter": "api_calls", "quantity": 3, "timestamp": "2026-06-08T11:00:00Z", "metadata": map[string]any{"region": "us-west-2"}},
		{"subject": "org_123", "meter": "api_calls", "quantity": 5, "timestamp": "2026-06-08T12:00:00Z", "metadata": map[string]any{"region": "us-east-1"}},
	}
	for _, event := range events {
		createUsage := requestJSON(t, router, http.MethodPost, "/v1/usages", event)
		if createUsage.Code != http.StatusCreated {
			t.Fatalf("create usage status = %d: %s", createUsage.Code, createUsage.Body.String())
		}
	}

	list := requestJSON(t, router, http.MethodGet, "/v1/usages?subject=org_123&meter=api_calls&from=2026-06-08T00:00:00Z&to=2026-06-09T00:00:00Z&bucket_size=day&group_by=region", nil)
	if list.Code != http.StatusOK {
		t.Fatalf("list usages status = %d, want %d: %s", list.Code, http.StatusOK, list.Body.String())
	}

	var buckets []usageListItemResponse
	decodeJSON(t, list, &buckets)
	if len(buckets) != 2 {
		t.Fatalf("bucket count = %d, want 2", len(buckets))
	}
	if buckets[0].Group["region"] != "us-east-1" || buckets[0].Quantity != 7 {
		t.Fatalf("first grouped bucket = %#v", buckets[0])
	}
	if buckets[1].Group["region"] != "us-west-2" || buckets[1].Quantity != 3 {
		t.Fatalf("second grouped bucket = %#v", buckets[1])
	}

	export := requestJSON(t, router, http.MethodGet, "/v1/usages/export?subject=org_123&meter=api_calls&from=2026-06-08T00:00:00Z&to=2026-06-09T00:00:00Z&bucket_size=day&group_by=region", nil)
	if export.Code != http.StatusOK {
		t.Fatalf("export status = %d, want %d: %s", export.Code, http.StatusOK, export.Body.String())
	}

	want := "bucket_start,subject,meter,bucket_size,aggregation,unit,quantity,region\n2026-06-08T00:00:00Z,org_123,api_calls,day,sum,call,7,us-east-1\n2026-06-08T00:00:00Z,org_123,api_calls,day,sum,call,3,us-west-2\n"
	if export.Body.String() != want {
		t.Fatalf("csv = %q, want %q", export.Body.String(), want)
	}
}

func TestUsageAPIRejectsInvalidLimit(t *testing.T) {
	router := newTestRouter()

	res := requestJSON(t, router, http.MethodGet, "/v1/usages?limit=zero", nil)
	if res.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d: %s", res.Code, http.StatusBadRequest, res.Body.String())
	}

	var errRes errorResponse
	decodeJSON(t, res, &errRes)
	if errRes.Error.Code != "invalid_limit" {
		t.Fatalf("error code = %q, want invalid_limit", errRes.Error.Code)
	}
}

func TestUsageAPIRejectsInvalidTimeQuery(t *testing.T) {
	router := newTestRouter()

	res := requestJSON(t, router, http.MethodGet, "/v1/usageevents?from=not-a-time", nil)
	if res.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d: %s", res.Code, http.StatusBadRequest, res.Body.String())
	}

	var errRes errorResponse
	decodeJSON(t, res, &errRes)
	if errRes.Error.Code != "invalid_from" || errRes.Error.Message != "from must be RFC3339" {
		t.Fatalf("error = %#v, want invalid_from/from must be RFC3339", errRes.Error)
	}
}

func TestUsageAPIRejectsInvalidDryRun(t *testing.T) {
	router := newTestRouter()

	res := requestJSON(t, router, http.MethodPost, "/v1/usageevents/prune?dry_run=maybe", nil)
	if res.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d: %s", res.Code, http.StatusBadRequest, res.Body.String())
	}

	var errRes errorResponse
	decodeJSON(t, res, &errRes)
	if errRes.Error.Code != "invalid_dry_run" || errRes.Error.Message != "dry_run must be true or false" {
		t.Fatalf("error = %#v, want invalid_dry_run/dry_run must be true or false", errRes.Error)
	}
}

func TestUsageAPIRejectsTooWideRange(t *testing.T) {
	router := newTestRouter()

	createMeter := requestJSON(t, router, http.MethodPost, "/v1/meters", map[string]any{
		"name":        "api_calls",
		"description": "API calls",
		"unit":        "call",
		"aggregation": "sum",
	})
	if createMeter.Code != http.StatusCreated {
		t.Fatalf("create meter status = %d: %s", createMeter.Code, createMeter.Body.String())
	}

	res := requestJSON(t, router, http.MethodGet, "/v1/usages?subject=org_123&meter=api_calls&from=2026-01-01T00:00:00Z&to=2026-02-02T00:00:00Z&bucket_size=hour", nil)
	if res.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d: %s", res.Code, http.StatusBadRequest, res.Body.String())
	}

	var errRes errorResponse
	decodeJSON(t, res, &errRes)
	if errRes.Error.Code != "invalid_input" {
		t.Fatalf("error code = %q, want invalid_input", errRes.Error.Code)
	}
}

func TestUsageAPIRejectsUnknownMeter(t *testing.T) {
	router := newTestRouter()

	res := requestJSON(t, router, http.MethodPost, "/v1/usages", map[string]any{
		"subject":  "org_123",
		"meter":    "missing",
		"quantity": 1,
	})
	if res.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d: %s", res.Code, http.StatusNotFound, res.Body.String())
	}

	var errRes errorResponse
	decodeJSON(t, res, &errRes)
	if errRes.Error.Code != "not_found" {
		t.Fatalf("error code = %q, want not_found", errRes.Error.Code)
	}
	if errRes.Error.Message == "" {
		t.Fatal("error message is empty")
	}
}

func TestUsageAPIRejectsMetadataOutsideMeterSchema(t *testing.T) {
	router := newTestRouter()

	createMeter := requestJSON(t, router, http.MethodPost, "/v1/meters", map[string]any{
		"name":            "api_calls",
		"description":     "API calls",
		"unit":            "call",
		"aggregation":     "sum",
		"metadata_schema": map[string]string{"region": "string"},
	})
	if createMeter.Code != http.StatusCreated {
		t.Fatalf("create meter status = %d: %s", createMeter.Code, createMeter.Body.String())
	}

	res := requestJSON(t, router, http.MethodPost, "/v1/usages", map[string]any{
		"subject":  "org_123",
		"meter":    "api_calls",
		"quantity": 1,
		"metadata": map[string]any{
			"regoin": "us-east-1",
		},
	})
	if res.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d: %s", res.Code, http.StatusBadRequest, res.Body.String())
	}

	var errRes errorResponse
	decodeJSON(t, res, &errRes)
	if errRes.Error.Code != "invalid_input" {
		t.Fatalf("error code = %q, want invalid_input", errRes.Error.Code)
	}
}

func TestMeterAPIDeleteWithUsageReturnsConflict(t *testing.T) {
	router := newTestRouter()

	createMeter := requestJSON(t, router, http.MethodPost, "/v1/meters", map[string]any{
		"name":        "api_calls",
		"description": "API calls",
		"unit":        "call",
		"aggregation": "sum",
	})
	if createMeter.Code != http.StatusCreated {
		t.Fatalf("create meter status = %d: %s", createMeter.Code, createMeter.Body.String())
	}
	var created meterResponse
	decodeJSON(t, createMeter, &created)

	createUsage := requestJSON(t, router, http.MethodPost, "/v1/usages", map[string]any{
		"subject":  "org_123",
		"meter":    "api_calls",
		"quantity": 1,
	})
	if createUsage.Code != http.StatusCreated {
		t.Fatalf("create usage status = %d: %s", createUsage.Code, createUsage.Body.String())
	}

	del := requestJSON(t, router, http.MethodDelete, "/v1/meters/"+created.ID, nil)
	if del.Code != http.StatusConflict {
		t.Fatalf("delete meter status = %d, want %d: %s", del.Code, http.StatusConflict, del.Body.String())
	}

	var errRes errorResponse
	decodeJSON(t, del, &errRes)
	if errRes.Error.Code != "conflict" {
		t.Fatalf("error code = %q, want conflict", errRes.Error.Code)
	}
}

func TestMeterAPIInvalidJSONErrorContract(t *testing.T) {
	router := newTestRouter()

	req := httptest.NewRequest(http.MethodPost, "/v1/meters", bytes.NewBufferString("{"))
	req.Header.Set("Content-Type", "application/json")
	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)

	if res.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d: %s", res.Code, http.StatusBadRequest, res.Body.String())
	}

	var errRes errorResponse
	decodeJSON(t, res, &errRes)
	if errRes.Error.Code != "invalid_json" {
		t.Fatalf("error code = %q, want invalid_json", errRes.Error.Code)
	}
	if errRes.Error.Message != "invalid JSON body" {
		t.Fatalf("error message = %q, want invalid JSON body", errRes.Error.Message)
	}
}

func TestMeterAPIRejectsTrailingJSON(t *testing.T) {
	router := newTestRouter()

	req := httptest.NewRequest(http.MethodPost, "/v1/meters", bytes.NewBufferString(`{"name":"api_calls","unit":"call"} {}`))
	req.Header.Set("Content-Type", "application/json")
	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)

	if res.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d: %s", res.Code, http.StatusBadRequest, res.Body.String())
	}

	var errRes errorResponse
	decodeJSON(t, res, &errRes)
	if errRes.Error.Code != "invalid_json" || errRes.Error.Message != "invalid JSON body" {
		t.Fatalf("error = %#v, want invalid_json/invalid JSON body", errRes.Error)
	}
}

func TestSystemStatsAPIContract(t *testing.T) {
	router := newTestRouter()

	createMeter := requestJSON(t, router, http.MethodPost, "/v1/meters", map[string]any{
		"name":                 "stats_events",
		"description":          "Stats events",
		"unit":                 "event",
		"aggregation":          "sum",
		"event_retention_days": 1,
	})
	if createMeter.Code != http.StatusCreated {
		t.Fatalf("create meter status = %d: %s", createMeter.Code, createMeter.Body.String())
	}

	for _, event := range []map[string]any{
		{"subject": "org_123", "meter": "stats_events", "quantity": 1, "timestamp": "2026-01-01T00:00:00Z"},
		{"subject": "org_123", "meter": "stats_events", "quantity": 2, "timestamp": time.Now().UTC().Format(time.RFC3339)},
	} {
		createUsage := requestJSON(t, router, http.MethodPost, "/v1/usages", event)
		if createUsage.Code != http.StatusCreated {
			t.Fatalf("create usage status = %d: %s", createUsage.Code, createUsage.Body.String())
		}
	}

	prune := requestJSON(t, router, http.MethodPost, "/v1/usageevents/prune", nil)
	if prune.Code != http.StatusOK {
		t.Fatalf("prune status = %d: %s", prune.Code, prune.Body.String())
	}

	stats := requestJSON(t, router, http.MethodGet, "/v1/system/stats", nil)
	if stats.Code != http.StatusOK {
		t.Fatalf("stats status = %d, want %d: %s", stats.Code, http.StatusOK, stats.Body.String())
	}

	var result systemStatsResponse
	decodeJSON(t, stats, &result)
	if result.Meters != 1 || result.UsageEvents != 1 || result.PruneRuns != 1 {
		t.Fatalf("stats = %#v", result)
	}
	if result.LastPruneRun == nil || result.LastPruneRun.ID == "" || result.LastPruneRun.Deleted != 1 || result.LastPruneRun.DryRun {
		t.Fatalf("last prune run = %#v", result.LastPruneRun)
	}
}

func TestUsageIngestionHistoryAPIContract(t *testing.T) {
	router := newTestRouter()

	createMeter := requestJSON(t, router, http.MethodPost, "/v1/meters", map[string]any{
		"name":        "ingested_events",
		"description": "Ingested events",
		"unit":        "event",
		"aggregation": "sum",
	})
	if createMeter.Code != http.StatusCreated {
		t.Fatalf("create meter status = %d: %s", createMeter.Code, createMeter.Body.String())
	}

	createUsage := requestJSON(t, router, http.MethodPost, "/v1/usages", map[string]any{
		"idempotency_key": "ingestion-single-1",
		"subject":         "org_123",
		"meter":           "ingested_events",
		"quantity":        1,
	})
	if createUsage.Code != http.StatusCreated {
		t.Fatalf("create usage status = %d: %s", createUsage.Code, createUsage.Body.String())
	}

	bulk := requestJSON(t, router, http.MethodPost, "/v1/usages/bulk", []map[string]any{
		{
			"idempotency_key": "ingestion-bulk-1",
			"subject":         "org_123",
			"meter":           "ingested_events",
			"quantity":        2,
		},
		{
			"idempotency_key": "ingestion-bulk-2",
			"subject":         "org_123",
			"meter":           "missing",
			"quantity":        3,
		},
	})
	if bulk.Code != http.StatusCreated {
		t.Fatalf("bulk status = %d: %s", bulk.Code, bulk.Body.String())
	}

	history := requestJSON(t, router, http.MethodGet, "/v1/usageingestions?limit=10", nil)
	if history.Code != http.StatusOK {
		t.Fatalf("ingestion history status = %d, want %d: %s", history.Code, http.StatusOK, history.Body.String())
	}

	var runs ingestionListResponse
	decodeJSON(t, history, &runs)
	if len(runs.Items) != 2 {
		t.Fatalf("ingestion runs = %#v", runs)
	}
	byKind := map[string]ingestionResponse{}
	for _, run := range runs.Items {
		byKind[run.Kind] = run
	}
	if byKind["bulk"].Accepted != 1 || byKind["bulk"].Duplicates != 0 || byKind["bulk"].Failed != 1 || byKind["bulk"].ID == "" || byKind["bulk"].CreatedAt == "" {
		t.Fatalf("bulk ingestion run = %#v", byKind["bulk"])
	}
	if byKind["single"].Accepted != 1 || byKind["single"].Duplicates != 0 || byKind["single"].Failed != 0 || byKind["single"].ID == "" || byKind["single"].CreatedAt == "" {
		t.Fatalf("single ingestion run = %#v", byKind["single"])
	}
}

func newTestRouter() http.Handler {
	store, err := sqlite.NewStore(context.Background(), ":memory:", config.DBPoolConfig{MaxOpenConns: 1})
	if err != nil {
		panic(err)
	}
	meterRepo := sqlite.NewMeterRepository(store)
	usageRepo := sqlite.NewUsageRepository(store)
	authRepo := sqlite.NewAuthRepository(store)
	authService := appauth.NewService(authRepo)
	meterService := appmeter.NewService(meterRepo, usageRepo)
	subjectService := appsubject.NewService(usageRepo)
	usageService := appusage.NewService(meterRepo, usageRepo, store)
	systemService := appsystem.NewService(meterRepo, usageRepo)

	router := chi.NewRouter()
	router.Route("/v1", func(r chi.Router) {
		httpauth.NewHandler(authService).RegisterRoutes(r)
		httpmeter.NewHandler(meterService).RegisterRoutes(r)
		httpsubject.NewHandler(subjectService).RegisterRoutes(r)
		httpusage.NewHandler(usageService).RegisterRoutes(r)
		httpsystem.NewHandler(systemService).RegisterRoutes(r)
	})

	return router
}

func requestJSON(t *testing.T, handler http.Handler, method string, path string, body any) *httptest.ResponseRecorder {
	return requestJSONWithHeaders(t, handler, method, path, body, nil)
}

func requestJSONWithHeaders(t *testing.T, handler http.Handler, method string, path string, body any, headers map[string]string) *httptest.ResponseRecorder {
	return requestJSONWithOptions(t, handler, method, path, body, headers, nil)
}

func requestJSONWithCookies(t *testing.T, handler http.Handler, method string, path string, body any, cookies []*http.Cookie) *httptest.ResponseRecorder {
	return requestJSONWithOptions(t, handler, method, path, body, nil, cookies)
}

func requestJSONWithOptions(t *testing.T, handler http.Handler, method string, path string, body any, headers map[string]string, cookies []*http.Cookie) *httptest.ResponseRecorder {
	t.Helper()

	var payload bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&payload).Encode(body); err != nil {
			t.Fatalf("encode request body: %v", err)
		}
	}

	req := httptest.NewRequest(method, path, &payload)
	req.Header.Set("Content-Type", "application/json")
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	for _, cookie := range cookies {
		req.AddCookie(cookie)
	}
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)

	return res
}

func decodeJSON(t *testing.T, res *httptest.ResponseRecorder, target any) {
	t.Helper()

	if err := json.NewDecoder(res.Body).Decode(target); err != nil {
		t.Fatalf("decode response body: %v; body = %s", err, res.Body.String())
	}
}

func findCookie(cookies []*http.Cookie, name string) *http.Cookie {
	for _, cookie := range cookies {
		if cookie.Name == name {
			return cookie
		}
	}
	return nil
}

type meterResponse struct {
	ID                 string            `json:"id"`
	Name               string            `json:"name"`
	Description        string            `json:"description"`
	Unit               string            `json:"unit"`
	Aggregation        string            `json:"aggregation"`
	MetadataSchema     map[string]string `json:"metadata_schema"`
	EventRetentionDays int               `json:"event_retention_days"`
	CreatedAt          string            `json:"created_at"`
}

type meterListResponse struct {
	Items      []meterResponse `json:"items"`
	NextCursor string          `json:"next_cursor"`
}

type meterStatsResponse struct {
	Meter              string `json:"meter"`
	UsageEvents        int    `json:"usage_events"`
	LastEventAt        string `json:"last_event_at"`
	EventRetentionDays int    `json:"retention_days"`
}

type meterStatsListResponse struct {
	Items      []meterStatsResponse `json:"items"`
	NextCursor string               `json:"next_cursor"`
}

type subjectStatsResponse struct {
	Subject     string `json:"subject"`
	UsageEvents int    `json:"usage_events"`
	Meters      int    `json:"meters"`
	LastEventAt string `json:"last_event_at"`
}

type subjectStatsListResponse struct {
	Items      []subjectStatsResponse `json:"items"`
	NextCursor string                 `json:"next_cursor"`
}

type usageResponse struct {
	ID             string         `json:"id"`
	IdempotencyKey string         `json:"idempotency_key"`
	Subject        string         `json:"subject"`
	Meter          string         `json:"meter"`
	Quantity       float64        `json:"quantity"`
	Timestamp      string         `json:"timestamp"`
	ReceivedAt     string         `json:"received_at"`
	Metadata       map[string]any `json:"metadata"`
}

type eventListResponse struct {
	Items      []usageResponse `json:"items"`
	NextCursor string          `json:"next_cursor"`
}

type bulkResponse struct {
	AcceptedCount  int             `json:"accepted"`
	DuplicateCount int             `json:"duplicates"`
	FailedCount    int             `json:"failed"`
	Accepted       []usageResponse `json:"accepted_items"`
	Duplicates     []usageResponse `json:"duplicate_items"`
	Failed         []bulkFailure   `json:"failed_items"`
}

type bulkFailure struct {
	Index   int    `json:"index"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

type pruneResponse struct {
	Deleted   int                  `json:"deleted"`
	DryRun    bool                 `json:"dry_run"`
	Meters    []pruneMeterResponse `json:"meters"`
	ID        string               `json:"id"`
	CreatedAt string               `json:"created_at"`
}

type pruneListResponse struct {
	Items      []pruneResponse `json:"items"`
	NextCursor string          `json:"next_cursor"`
}

type pruneMeterResponse struct {
	Meter   string `json:"meter"`
	Before  string `json:"before"`
	Deleted int    `json:"deleted"`
}

type systemStatsResponse struct {
	Meters       int                   `json:"meters"`
	UsageEvents  int                   `json:"usage_events"`
	PruneRuns    int                   `json:"prune_runs"`
	LastPruneRun *lastPruneRunResponse `json:"last_prune_run"`
}

type lastPruneRunResponse struct {
	ID        string `json:"id"`
	Deleted   int    `json:"deleted"`
	DryRun    bool   `json:"dry_run"`
	CreatedAt string `json:"created_at"`
}

type ingestionListResponse struct {
	Items      []ingestionResponse `json:"items"`
	NextCursor string              `json:"next_cursor"`
}

type ingestionResponse struct {
	ID         string `json:"id"`
	Kind       string `json:"kind"`
	Accepted   int    `json:"accepted"`
	Duplicates int    `json:"duplicates"`
	Failed     int    `json:"failed"`
	CreatedAt  string `json:"created_at"`
}

type usageListItemResponse struct {
	Subject     string            `json:"subject"`
	Meter       string            `json:"meter"`
	BucketSize  string            `json:"bucket_size"`
	BucketStart string            `json:"bucket_start"`
	Aggregation string            `json:"aggregation"`
	Unit        string            `json:"unit"`
	Quantity    float64           `json:"quantity"`
	Group       map[string]string `json:"group"`
}

type errorResponse struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

type authUserResponse struct {
	ID        string `json:"id"`
	Email     string `json:"email"`
	CreatedAt string `json:"created_at"`
}

type authSessionResponse struct {
	ExpiresAt string           `json:"expires_at"`
	User      authUserResponse `json:"user"`
}

type authCurrentSessionResponse struct {
	User authUserResponse `json:"user"`
}
