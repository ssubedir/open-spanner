package http_test

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	appauth "github.com/ssubedir/open-spanner/internal/auth"
	"github.com/ssubedir/open-spanner/internal/config"
	httpauth "github.com/ssubedir/open-spanner/internal/metering/adapters/http/auth"
	httpentitlement "github.com/ssubedir/open-spanner/internal/metering/adapters/http/entitlement"
	httpmeter "github.com/ssubedir/open-spanner/internal/metering/adapters/http/meter"
	httpsubject "github.com/ssubedir/open-spanner/internal/metering/adapters/http/subject"
	httpsystem "github.com/ssubedir/open-spanner/internal/metering/adapters/http/system"
	httpusage "github.com/ssubedir/open-spanner/internal/metering/adapters/http/usage"
	"github.com/ssubedir/open-spanner/internal/metering/adapters/sqlite"
	appentitlement "github.com/ssubedir/open-spanner/internal/metering/app/entitlement"
	appmeter "github.com/ssubedir/open-spanner/internal/metering/app/meter"
	appsubject "github.com/ssubedir/open-spanner/internal/metering/app/subject"
	appsystem "github.com/ssubedir/open-spanner/internal/metering/app/system"
	appusage "github.com/ssubedir/open-spanner/internal/metering/app/usage"
	domainusage "github.com/ssubedir/open-spanner/internal/metering/domain/usage"
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

	createKey := requestJSONWithCookies(t, router, http.MethodPost, "/v1/auth/api-keys", map[string]any{
		"name": "sdk",
	}, cookies)
	if createKey.Code != http.StatusCreated {
		t.Fatalf("create api key status = %d, want %d: %s", createKey.Code, http.StatusCreated, createKey.Body.String())
	}
	var createdKey authAPIKeyCreateResponse
	decodeJSON(t, createKey, &createdKey)
	if createdKey.ID == "" || createdKey.Name != "sdk" || createdKey.Prefix == "" || createdKey.Key == "" {
		t.Fatalf("created api key = %#v", createdKey)
	}
	if strings.Contains(createKey.Body.String(), "password") {
		t.Fatalf("api key response exposed password material: %s", createKey.Body.String())
	}

	listKeys := requestJSONWithCookies(t, router, http.MethodGet, "/v1/auth/api-keys", nil, cookies)
	if listKeys.Code != http.StatusOK {
		t.Fatalf("list api keys status = %d, want %d: %s", listKeys.Code, http.StatusOK, listKeys.Body.String())
	}
	var keyList authAPIKeyListResponse
	decodeJSON(t, listKeys, &keyList)
	if len(keyList.Items) != 1 || keyList.Items[0].ID != createdKey.ID || keyList.Items[0].Key != "" {
		t.Fatalf("api key list = %#v", keyList)
	}

	deleteKey := requestJSONWithCookies(t, router, http.MethodDelete, "/v1/auth/api-keys/"+createdKey.ID, nil, cookies)
	if deleteKey.Code != http.StatusNoContent {
		t.Fatalf("delete api key status = %d, want %d: %s", deleteKey.Code, http.StatusNoContent, deleteKey.Body.String())
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

func TestAuthenticatedWorkspaceIsolation(t *testing.T) {
	router := newProtectedTestRouter()

	userACookies := registerAndLogin(t, router, "tenant-a@example.com", "strong-password-a")
	userBCookies := registerAndLogin(t, router, "tenant-b@example.com", "strong-password-b")

	createMeterA := requestJSONWithCookies(t, router, http.MethodPost, "/v1/meters", map[string]any{
		"name":        "tenant_requests",
		"description": "Tenant scoped requests",
		"unit":        "request",
		"aggregation": "sum",
	}, userACookies)
	if createMeterA.Code != http.StatusCreated {
		t.Fatalf("create user A meter status = %d, want %d: %s", createMeterA.Code, http.StatusCreated, createMeterA.Body.String())
	}
	var meterA meterResponse
	decodeJSON(t, createMeterA, &meterA)

	createUsageA := requestJSONWithCookies(t, router, http.MethodPost, "/v1/usages", map[string]any{
		"idempotency_key": "shared-idempotency-key",
		"subject":         "org_a",
		"meter":           "tenant_requests",
		"quantity":        7,
		"timestamp":       "2026-06-20T12:00:00Z",
	}, userACookies)
	if createUsageA.Code != http.StatusCreated {
		t.Fatalf("create user A usage status = %d, want %d: %s", createUsageA.Code, http.StatusCreated, createUsageA.Body.String())
	}

	listMetersB := requestJSONWithCookies(t, router, http.MethodGet, "/v1/meters", nil, userBCookies)
	if listMetersB.Code != http.StatusOK {
		t.Fatalf("list user B meters status = %d, want %d: %s", listMetersB.Code, http.StatusOK, listMetersB.Body.String())
	}
	var metersB meterListResponse
	decodeJSON(t, listMetersB, &metersB)
	if len(metersB.Items) != 0 {
		t.Fatalf("user B meters = %#v, want empty list", metersB.Items)
	}

	getMeterB := requestJSONWithCookies(t, router, http.MethodGet, "/v1/meters/"+meterA.ID, nil, userBCookies)
	if getMeterB.Code != http.StatusNotFound {
		t.Fatalf("get user A meter as user B status = %d, want %d: %s", getMeterB.Code, http.StatusNotFound, getMeterB.Body.String())
	}

	createMeterB := requestJSONWithCookies(t, router, http.MethodPost, "/v1/meters", map[string]any{
		"name":        "tenant_requests",
		"description": "Same name in another workspace",
		"unit":        "request",
		"aggregation": "sum",
	}, userBCookies)
	if createMeterB.Code != http.StatusCreated {
		t.Fatalf("create user B meter status = %d, want %d: %s", createMeterB.Code, http.StatusCreated, createMeterB.Body.String())
	}

	listUsageB := requestJSONWithCookies(t, router, http.MethodGet, "/v1/usages?subject=org_a&meter=tenant_requests&from=2026-06-20T00:00:00Z&to=2026-06-21T00:00:00Z&bucket_size=day", nil, userBCookies)
	if listUsageB.Code != http.StatusOK {
		t.Fatalf("list user B usage status = %d, want %d: %s", listUsageB.Code, http.StatusOK, listUsageB.Body.String())
	}
	var usageB []usageListItemResponse
	decodeJSON(t, listUsageB, &usageB)
	if len(usageB) != 0 {
		t.Fatalf("user B usage = %#v, want empty list", usageB)
	}

	createUsageB := requestJSONWithCookies(t, router, http.MethodPost, "/v1/usages", map[string]any{
		"idempotency_key": "shared-idempotency-key",
		"subject":         "org_b",
		"meter":           "tenant_requests",
		"quantity":        3,
		"timestamp":       "2026-06-20T12:00:00Z",
	}, userBCookies)
	if createUsageB.Code != http.StatusCreated {
		t.Fatalf("create user B usage status = %d, want %d: %s", createUsageB.Code, http.StatusCreated, createUsageB.Body.String())
	}
}

func TestMeterAPIContract(t *testing.T) {
	router := newTestRouter()

	create := requestJSON(t, router, http.MethodPost, "/v1/meters", map[string]any{
		"name":        "api_calls",
		"description": "API calls",
		"unit":        "call",
		"aggregation": "sum",
		"dimensions": []map[string]any{
			{
				"name":         "region",
				"display_name": "Region",
				"description":  "Deployment region",
				"type":         "string",
				"required":     true,
				"deprecated":   true,
			},
		},
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
	if len(created.Dimensions) != 1 || created.Dimensions[0].DisplayName != "Region" || !created.Dimensions[0].Deprecated {
		t.Fatalf("created meter dimensions = %#v", created.Dimensions)
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
		"description":          "Updated API calls",
		"unit":                 "request",
		"aggregation":          "count",
		"event_retention_days": 365,
		"dimensions": []map[string]any{
			{
				"name":         "plan",
				"display_name": "Plan",
				"description":  "Billing plan",
				"type":         "string",
				"required":     false,
				"deprecated":   true,
			},
		},
	})
	if update.Code != http.StatusOK {
		t.Fatalf("update meter status = %d, want %d: %s", update.Code, http.StatusOK, update.Body.String())
	}
	var updated meterResponse
	decodeJSON(t, update, &updated)
	if updated.Description != "Updated API calls" || updated.Name != created.Name || updated.Unit != "request" || updated.Aggregation != "count" || updated.EventRetentionDays != 365 || len(updated.Dimensions) != 1 || updated.Dimensions[0].DisplayName != "Plan" || updated.Dimensions[0].Required || !updated.Dimensions[0].Deprecated {
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

func TestEntitlementCheckAPIProvidesQuotaMetadata(t *testing.T) {
	router := newTestRouter()

	createMeter := requestJSON(t, router, http.MethodPost, "/v1/meters", map[string]any{
		"name":        "quota_api_calls",
		"description": "API calls governed by plan quota",
		"unit":        "call",
		"aggregation": "sum",
	})
	if createMeter.Code != http.StatusCreated {
		t.Fatalf("create meter status = %d, want %d: %s", createMeter.Code, http.StatusCreated, createMeter.Body.String())
	}

	createPlan := requestJSON(t, router, http.MethodPost, "/v1/plans", map[string]any{
		"name":        "Pro",
		"description": "Test quota plan",
		"limits": []map[string]any{
			{
				"meter":           "quota_api_calls",
				"period":          "month",
				"limit":           10,
				"warning_percent": 80,
			},
		},
	})
	if createPlan.Code != http.StatusCreated {
		t.Fatalf("create plan status = %d, want %d: %s", createPlan.Code, http.StatusCreated, createPlan.Body.String())
	}
	var plan entitlementPlanResponse
	decodeJSON(t, createPlan, &plan)
	if plan.ID == "" || len(plan.Limits) != 1 {
		t.Fatalf("created plan = %#v", plan)
	}

	assign := requestJSON(t, router, http.MethodPut, "/v1/plans/subjects/org_quota", map[string]any{
		"plan_id": plan.ID,
	})
	if assign.Code != http.StatusOK {
		t.Fatalf("assign subject status = %d, want %d: %s", assign.Code, http.StatusOK, assign.Body.String())
	}

	initialCheck := requestJSON(t, router, http.MethodPost, "/v1/entitlements/check", map[string]any{
		"subject":  "org_quota",
		"meter":    "quota_api_calls",
		"quantity": 3,
	})
	if initialCheck.Code != http.StatusOK {
		t.Fatalf("initial check status = %d, want %d: %s", initialCheck.Code, http.StatusOK, initialCheck.Body.String())
	}
	var initial entitlementCheckResponse
	decodeJSON(t, initialCheck, &initial)
	if !initial.Allowed || initial.State != "ok" || initial.Current != 0 || initial.Remaining != 7 || initial.Overage != 0 || initial.PeriodResetAt == "" || initial.RetryAfterSeconds != 0 {
		t.Fatalf("initial entitlement check = %#v", initial)
	}

	createUsage := requestJSON(t, router, http.MethodPost, "/v1/usages", map[string]any{
		"subject":  "org_quota",
		"meter":    "quota_api_calls",
		"quantity": 8,
	})
	if createUsage.Code != http.StatusCreated {
		t.Fatalf("create usage status = %d, want %d: %s", createUsage.Code, http.StatusCreated, createUsage.Body.String())
	}

	warningCheck := requestJSON(t, router, http.MethodPost, "/v1/entitlements/check", map[string]any{
		"subject":  "org_quota",
		"meter":    "quota_api_calls",
		"quantity": 1,
	})
	if warningCheck.Code != http.StatusOK {
		t.Fatalf("warning check status = %d, want %d: %s", warningCheck.Code, http.StatusOK, warningCheck.Body.String())
	}
	var warning entitlementCheckResponse
	decodeJSON(t, warningCheck, &warning)
	if !warning.Allowed || warning.State != "warning" || warning.Current != 8 || warning.Remaining != 1 || warning.Overage != 0 || warning.Message == "" {
		t.Fatalf("warning entitlement check = %#v", warning)
	}

	exceededCheck := requestJSON(t, router, http.MethodPost, "/v1/entitlements/check", map[string]any{
		"subject":  "org_quota",
		"meter":    "quota_api_calls",
		"quantity": 3,
	})
	if exceededCheck.Code != http.StatusOK {
		t.Fatalf("exceeded check status = %d, want %d: %s", exceededCheck.Code, http.StatusOK, exceededCheck.Body.String())
	}
	var exceeded entitlementCheckResponse
	decodeJSON(t, exceededCheck, &exceeded)
	if exceeded.Allowed || exceeded.State != "exceeded" || exceeded.Current != 8 || exceeded.Remaining != 0 || exceeded.Overage != 1 || exceeded.PeriodResetAt == "" || exceeded.RetryAfterSeconds <= 0 {
		t.Fatalf("exceeded entitlement check = %#v", exceeded)
	}

	progress := requestJSON(t, router, http.MethodGet, "/v1/plans/subjects/org_quota/progress", nil)
	if progress.Code != http.StatusOK {
		t.Fatalf("progress status = %d, want %d: %s", progress.Code, http.StatusOK, progress.Body.String())
	}
	var progressResult entitlementProgressResponse
	decodeJSON(t, progress, &progressResult)
	if len(progressResult.Items) != 1 || progressResult.Items[0].Current != 8 || progressResult.Items[0].Remaining != 2 || progressResult.Items[0].Overage != 0 || progressResult.Items[0].PeriodResetAt == "" {
		t.Fatalf("progress result = %#v", progressResult)
	}
}

func TestUsageAPIContract(t *testing.T) {
	router := newTestRouter()

	createMeter := requestJSON(t, router, http.MethodPost, "/v1/meters", map[string]any{
		"name":        "tokens",
		"description": "Tokens",
		"unit":        "token",
		"aggregation": "sum",
		"dimensions":  meterDimensionsFromSchema(map[string]string{"region": "string"}),
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

	bucketExport := requestJSON(t, router, http.MethodPost, "/v1/usages/export", map[string]any{
		"subject":     "org_123",
		"meter":       "tokens",
		"from":        "2026-06-08T00:00:00Z",
		"to":          "2026-06-09T00:00:00Z",
		"bucket_size": "day",
		"group_by":    []string{"region"},
		"filter": map[string]any{
			"type": "group",
			"op":   "and",
			"rules": []map[string]any{
				{
					"type":  "condition",
					"field": "metadata.region",
					"op":    "eq",
					"value": "us-east-1",
				},
				{
					"type":  "condition",
					"field": "quantity",
					"op":    "gte",
					"value": 10,
				},
			},
		},
	})
	if bucketExport.Code != http.StatusOK {
		t.Fatalf("filtered export status = %d, want %d: %s", bucketExport.Code, http.StatusOK, bucketExport.Body.String())
	}
	wantBucketExport := "bucket_start,subject,meter,bucket_size,aggregation,unit,quantity,region\n2026-06-08T00:00:00Z,org_123,tokens,day,sum,token,11,us-east-1\n"
	if bucketExport.Body.String() != wantBucketExport {
		t.Fatalf("filtered export csv = %q, want %q", bucketExport.Body.String(), wantBucketExport)
	}

	eventExport := requestJSON(t, router, http.MethodPost, "/v1/usageevents/export", map[string]any{
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
	if eventExport.Code != http.StatusOK {
		t.Fatalf("filtered event export status = %d, want %d: %s", eventExport.Code, http.StatusOK, eventExport.Body.String())
	}
	eventRecords, err := csv.NewReader(bytes.NewReader(eventExport.Body.Bytes())).ReadAll()
	if err != nil {
		t.Fatalf("read filtered event csv: %v", err)
	}
	if len(eventRecords) != 2 || eventRecords[1][2] != "org_123" || eventRecords[1][3] != "tokens" || eventRecords[1][4] != "11" || !strings.Contains(eventRecords[1][5], "us-east-1") {
		t.Fatalf("filtered event csv records = %#v", eventRecords)
	}
}

func TestUsageDimensionValuesAPIContract(t *testing.T) {
	router := newTestRouter()

	createMeter := requestJSON(t, router, http.MethodPost, "/v1/meters", map[string]any{
		"name":        "requests",
		"description": "Requests",
		"unit":        "request",
		"aggregation": "sum",
		"dimensions":  meterDimensionsFromSchema(map[string]string{"region": "string"}),
	})
	if createMeter.Code != http.StatusCreated {
		t.Fatalf("create meter status = %d: %s", createMeter.Code, createMeter.Body.String())
	}

	for _, event := range []map[string]any{
		{
			"idempotency_key": "dimension-1",
			"subject":         "org_123",
			"meter":           "requests",
			"quantity":        1,
			"timestamp":       "2026-06-08T10:00:00Z",
			"metadata":        map[string]any{"region": "us-east-1"},
		},
		{
			"idempotency_key": "dimension-2",
			"subject":         "org_123",
			"meter":           "requests",
			"quantity":        1,
			"timestamp":       "2026-06-08T11:00:00Z",
			"metadata":        map[string]any{"region": "us-west-2"},
		},
		{
			"idempotency_key": "dimension-3",
			"subject":         "org_123",
			"meter":           "requests",
			"quantity":        1,
			"timestamp":       "2026-06-08T12:00:00Z",
			"metadata":        map[string]any{"region": "us-east-1"},
		},
		{
			"idempotency_key": "dimension-4",
			"subject":         "org_456",
			"meter":           "requests",
			"quantity":        1,
			"timestamp":       "2026-06-08T13:00:00Z",
			"metadata":        map[string]any{"region": "us-central-1"},
		},
	} {
		createUsage := requestJSON(t, router, http.MethodPost, "/v1/usages", event)
		if createUsage.Code != http.StatusCreated {
			t.Fatalf("create usage status = %d: %s", createUsage.Code, createUsage.Body.String())
		}
	}

	list := requestJSON(t, router, http.MethodGet, "/v1/usages/dimensions?meter=requests&field=region&subject=org_123&from=2026-06-08T00:00:00Z&to=2026-06-09T00:00:00Z", nil)
	if list.Code != http.StatusOK {
		t.Fatalf("dimension values status = %d, want %d: %s", list.Code, http.StatusOK, list.Body.String())
	}

	var values dimensionValueListResponse
	decodeJSON(t, list, &values)
	if len(values.Items) != 2 {
		t.Fatalf("dimension values = %#v, want 2 items", values.Items)
	}
	if values.Items[0].Field != "region" || values.Items[0].Value != "us-east-1" || values.Items[0].UsageEvents != 2 {
		t.Fatalf("first dimension value = %#v", values.Items[0])
	}
	if values.Items[1].Field != "region" || values.Items[1].Value != "us-west-2" || values.Items[1].UsageEvents != 1 {
		t.Fatalf("second dimension value = %#v", values.Items[1])
	}

	unknown := requestJSON(t, router, http.MethodGet, "/v1/usages/dimensions?meter=requests&field=plan", nil)
	if unknown.Code != http.StatusBadRequest {
		t.Fatalf("unknown dimension status = %d, want %d: %s", unknown.Code, http.StatusBadRequest, unknown.Body.String())
	}
}

func TestUsageBreakdownAPIContract(t *testing.T) {
	router := newTestRouter()

	createMeter := requestJSON(t, router, http.MethodPost, "/v1/meters", map[string]any{
		"name":        "requests",
		"description": "Requests",
		"unit":        "request",
		"aggregation": "sum",
		"dimensions":  meterDimensionsFromSchema(map[string]string{"endpoint": "string"}),
	})
	if createMeter.Code != http.StatusCreated {
		t.Fatalf("create meter status = %d: %s", createMeter.Code, createMeter.Body.String())
	}

	for _, event := range []map[string]any{
		{
			"idempotency_key": "breakdown-1",
			"subject":         "org_123",
			"meter":           "requests",
			"quantity":        2,
			"timestamp":       "2026-06-08T10:00:00Z",
			"metadata":        map[string]any{"endpoint": "/orders"},
		},
		{
			"idempotency_key": "breakdown-2",
			"subject":         "org_123",
			"meter":           "requests",
			"quantity":        3,
			"timestamp":       "2026-06-08T11:00:00Z",
			"metadata":        map[string]any{"endpoint": "/users"},
		},
		{
			"idempotency_key": "breakdown-3",
			"subject":         "org_456",
			"meter":           "requests",
			"quantity":        7,
			"timestamp":       "2026-06-08T12:00:00Z",
			"metadata":        map[string]any{"endpoint": "/orders"},
		},
		{
			"idempotency_key": "breakdown-4",
			"subject":         "org_789",
			"meter":           "requests",
			"quantity":        1,
			"timestamp":       "2026-06-08T13:00:00Z",
			"metadata":        map[string]any{"endpoint": "/users"},
		},
	} {
		createUsage := requestJSON(t, router, http.MethodPost, "/v1/usages", event)
		if createUsage.Code != http.StatusCreated {
			t.Fatalf("create usage status = %d: %s", createUsage.Code, createUsage.Body.String())
		}
	}

	subjectBreakdown := requestJSON(t, router, http.MethodPost, "/v1/usages/breakdowns/search", map[string]any{
		"meter": "requests",
		"field": "subject",
		"from":  "2026-06-08T00:00:00Z",
		"to":    "2026-06-09T00:00:00Z",
		"limit": 10,
	})
	if subjectBreakdown.Code != http.StatusOK {
		t.Fatalf("subject breakdown status = %d, want %d: %s", subjectBreakdown.Code, http.StatusOK, subjectBreakdown.Body.String())
	}

	var subjects breakdownListResponse
	decodeJSON(t, subjectBreakdown, &subjects)
	if len(subjects.Items) != 3 {
		t.Fatalf("subject breakdowns = %#v, want three items", subjects.Items)
	}
	if subjects.Items[0].Field != "subject" || subjects.Items[0].Value != "org_456" || subjects.Items[0].Quantity != 7 || subjects.Items[0].UsageEvents != 1 || subjects.Items[0].Aggregation != "sum" || subjects.Items[0].Unit != "request" {
		t.Fatalf("first subject breakdown = %#v", subjects.Items[0])
	}
	if subjects.Items[1].Value != "org_123" || subjects.Items[1].Quantity != 5 || subjects.Items[1].UsageEvents != 2 {
		t.Fatalf("second subject breakdown = %#v", subjects.Items[1])
	}

	endpointBreakdown := requestJSON(t, router, http.MethodPost, "/v1/usages/breakdowns/search", map[string]any{
		"subject": "org_123",
		"meter":   "requests",
		"field":   "metadata.endpoint",
		"from":    "2026-06-08T00:00:00Z",
		"to":      "2026-06-09T00:00:00Z",
		"limit":   10,
	})
	if endpointBreakdown.Code != http.StatusOK {
		t.Fatalf("endpoint breakdown status = %d, want %d: %s", endpointBreakdown.Code, http.StatusOK, endpointBreakdown.Body.String())
	}

	var endpoints breakdownListResponse
	decodeJSON(t, endpointBreakdown, &endpoints)
	if len(endpoints.Items) != 2 {
		t.Fatalf("endpoint breakdowns = %#v, want two items", endpoints.Items)
	}
	if endpoints.Items[0].Field != "endpoint" || endpoints.Items[0].Value != "/users" || endpoints.Items[0].Quantity != 3 || endpoints.Items[0].UsageEvents != 1 {
		t.Fatalf("first endpoint breakdown = %#v", endpoints.Items[0])
	}
	if endpoints.Items[1].Value != "/orders" || endpoints.Items[1].Quantity != 2 || endpoints.Items[1].UsageEvents != 1 {
		t.Fatalf("second endpoint breakdown = %#v", endpoints.Items[1])
	}

	unknown := requestJSON(t, router, http.MethodPost, "/v1/usages/breakdowns/search", map[string]any{
		"meter": "requests",
		"field": "plan",
		"from":  "2026-06-08T00:00:00Z",
		"to":    "2026-06-09T00:00:00Z",
	})
	if unknown.Code != http.StatusBadRequest {
		t.Fatalf("unknown breakdown status = %d, want %d: %s", unknown.Code, http.StatusBadRequest, unknown.Body.String())
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
		"name":        "api_calls",
		"description": "API calls",
		"unit":        "call",
		"aggregation": "sum",
		"dimensions":  meterDimensionsFromSchema(map[string]string{"region": "string"}),
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
		"name":        "api_calls",
		"description": "API Calls",
		"unit":        "call",
		"aggregation": "sum",
		"dimensions":  meterDimensionsFromSchema(map[string]string{"region": "string"}),
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
		"name":        "api_calls",
		"description": "API calls",
		"unit":        "call",
		"aggregation": "sum",
		"dimensions":  meterDimensionsFromSchema(map[string]string{"region": "string"}),
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

func TestUsageExportsRejectOverLimit(t *testing.T) {
	router := newTestRouter()
	overLimit := domainusage.MaxLimit + 1
	overLimitValue := strconv.Itoa(overLimit)
	wantMessage := "limit must be less than or equal to " + strconv.Itoa(domainusage.MaxLimit)
	cases := []struct {
		name   string
		method string
		path   string
		body   any
	}{
		{
			name:   "bucket query export",
			method: http.MethodGet,
			path:   "/v1/usages/export?meter=api_calls&from=2026-06-08T00:00:00Z&to=2026-06-09T00:00:00Z&bucket_size=day&limit=" + overLimitValue,
		},
		{
			name:   "bucket filtered export",
			method: http.MethodPost,
			path:   "/v1/usages/export",
			body: map[string]any{
				"meter":       "api_calls",
				"from":        "2026-06-08T00:00:00Z",
				"to":          "2026-06-09T00:00:00Z",
				"bucket_size": "day",
				"limit":       overLimit,
			},
		},
		{
			name:   "event query export",
			method: http.MethodGet,
			path:   "/v1/usageevents/export?limit=" + overLimitValue,
		},
		{
			name:   "event filtered export",
			method: http.MethodPost,
			path:   "/v1/usageevents/export",
			body:   map[string]any{"limit": overLimit},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res := requestJSON(t, router, tc.method, tc.path, tc.body)
			if res.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want %d: %s", res.Code, http.StatusBadRequest, res.Body.String())
			}

			var errRes errorResponse
			decodeJSON(t, res, &errRes)
			if errRes.Error.Code != "invalid_limit" || errRes.Error.Message != wantMessage {
				t.Fatalf("error = %#v, want invalid_limit max message", errRes.Error)
			}
		})
	}
}

func TestUsageAPIGroupByMetadata(t *testing.T) {
	router := newTestRouter()

	createMeter := requestJSON(t, router, http.MethodPost, "/v1/meters", map[string]any{
		"name":        "api_calls",
		"description": "API calls",
		"unit":        "call",
		"aggregation": "sum",
		"dimensions":  meterDimensionsFromSchema(map[string]string{"plan": "string", "region": "string"}),
	})
	if createMeter.Code != http.StatusCreated {
		t.Fatalf("create meter status = %d: %s", createMeter.Code, createMeter.Body.String())
	}

	events := []map[string]any{
		{"subject": "org_123", "meter": "api_calls", "quantity": 2, "timestamp": "2026-06-08T10:00:00Z", "metadata": map[string]any{"plan": "free", "region": "us-east-1"}},
		{"subject": "org_123", "meter": "api_calls", "quantity": 3, "timestamp": "2026-06-08T11:00:00Z", "metadata": map[string]any{"plan": "pro", "region": "us-east-1"}},
		{"subject": "org_123", "meter": "api_calls", "quantity": 5, "timestamp": "2026-06-08T12:00:00Z", "metadata": map[string]any{"plan": "free", "region": "us-west-2"}},
		{"subject": "org_123", "meter": "api_calls", "quantity": 7, "timestamp": "2026-06-08T13:00:00Z", "metadata": map[string]any{"plan": "free", "region": "us-east-1"}},
	}
	for _, event := range events {
		createUsage := requestJSON(t, router, http.MethodPost, "/v1/usages", event)
		if createUsage.Code != http.StatusCreated {
			t.Fatalf("create usage status = %d: %s", createUsage.Code, createUsage.Body.String())
		}
	}

	list := requestJSON(t, router, http.MethodGet, "/v1/usages?subject=org_123&meter=api_calls&from=2026-06-08T00:00:00Z&to=2026-06-09T00:00:00Z&bucket_size=day&group_by=region&group_by=plan", nil)
	if list.Code != http.StatusOK {
		t.Fatalf("list usages status = %d, want %d: %s", list.Code, http.StatusOK, list.Body.String())
	}

	var buckets []usageListItemResponse
	decodeJSON(t, list, &buckets)
	if len(buckets) != 3 {
		t.Fatalf("bucket count = %d, want 3", len(buckets))
	}
	if buckets[0].Group["region"] != "us-east-1" || buckets[0].Group["plan"] != "free" || buckets[0].Quantity != 9 {
		t.Fatalf("first grouped bucket = %#v", buckets[0])
	}
	if buckets[1].Group["region"] != "us-east-1" || buckets[1].Group["plan"] != "pro" || buckets[1].Quantity != 3 {
		t.Fatalf("second grouped bucket = %#v", buckets[1])
	}
	if buckets[2].Group["region"] != "us-west-2" || buckets[2].Group["plan"] != "free" || buckets[2].Quantity != 5 {
		t.Fatalf("third grouped bucket = %#v", buckets[2])
	}

	search := requestJSON(t, router, http.MethodPost, "/v1/usages/search", map[string]any{
		"subject":     "org_123",
		"meter":       "api_calls",
		"from":        "2026-06-08T00:00:00Z",
		"to":          "2026-06-09T00:00:00Z",
		"bucket_size": "day",
		"group_by":    []string{"region"},
	})
	if search.Code != http.StatusOK {
		t.Fatalf("search usages status = %d, want %d: %s", search.Code, http.StatusOK, search.Body.String())
	}
	var regionBuckets []usageListItemResponse
	decodeJSON(t, search, &regionBuckets)
	if len(regionBuckets) != 2 || regionBuckets[0].Group["region"] != "us-east-1" || regionBuckets[0].Quantity != 12 || regionBuckets[1].Group["region"] != "us-west-2" || regionBuckets[1].Quantity != 5 {
		t.Fatalf("region grouped buckets = %#v", regionBuckets)
	}

	stringSearch := requestJSON(t, router, http.MethodPost, "/v1/usages/search", map[string]any{
		"subject":     "org_123",
		"meter":       "api_calls",
		"from":        "2026-06-08T00:00:00Z",
		"to":          "2026-06-09T00:00:00Z",
		"bucket_size": "day",
		"group_by":    "region",
	})
	if stringSearch.Code != http.StatusBadRequest {
		t.Fatalf("string group_by status = %d, want %d: %s", stringSearch.Code, http.StatusBadRequest, stringSearch.Body.String())
	}

	arraySearch := requestJSON(t, router, http.MethodPost, "/v1/usages/search", map[string]any{
		"subject":     "org_123",
		"meter":       "api_calls",
		"from":        "2026-06-08T00:00:00Z",
		"to":          "2026-06-09T00:00:00Z",
		"bucket_size": "day",
		"group_by":    []string{"region", "plan"},
	})
	if arraySearch.Code != http.StatusOK {
		t.Fatalf("array search usages status = %d, want %d: %s", arraySearch.Code, http.StatusOK, arraySearch.Body.String())
	}
	var arrayBuckets []usageListItemResponse
	decodeJSON(t, arraySearch, &arrayBuckets)
	if len(arrayBuckets) != 3 || arrayBuckets[0].Group["plan"] != "free" || arrayBuckets[0].Quantity != 9 {
		t.Fatalf("array grouped buckets = %#v", arrayBuckets)
	}

	export := requestJSON(t, router, http.MethodGet, "/v1/usages/export?subject=org_123&meter=api_calls&from=2026-06-08T00:00:00Z&to=2026-06-09T00:00:00Z&bucket_size=day&group_by=region,plan", nil)
	if export.Code != http.StatusOK {
		t.Fatalf("export status = %d, want %d: %s", export.Code, http.StatusOK, export.Body.String())
	}

	want := "bucket_start,subject,meter,bucket_size,aggregation,unit,quantity,region,plan\n2026-06-08T00:00:00Z,org_123,api_calls,day,sum,call,9,us-east-1,free\n2026-06-08T00:00:00Z,org_123,api_calls,day,sum,call,3,us-east-1,pro\n2026-06-08T00:00:00Z,org_123,api_calls,day,sum,call,5,us-west-2,free\n"
	if export.Body.String() != want {
		t.Fatalf("csv = %q, want %q", export.Body.String(), want)
	}
}

func TestUsageAPIAggregatesAcrossSubjects(t *testing.T) {
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

	events := []map[string]any{
		{"subject": "org_123", "meter": "api_calls", "quantity": 2, "timestamp": "2026-06-08T10:00:00Z", "metadata": map[string]any{}},
		{"subject": "org_123", "meter": "api_calls", "quantity": 3, "timestamp": "2026-06-08T11:00:00Z", "metadata": map[string]any{}},
		{"subject": "org_456", "meter": "api_calls", "quantity": 5, "timestamp": "2026-06-08T12:00:00Z", "metadata": map[string]any{}},
	}
	for _, event := range events {
		createUsage := requestJSON(t, router, http.MethodPost, "/v1/usages", event)
		if createUsage.Code != http.StatusCreated {
			t.Fatalf("create usage status = %d: %s", createUsage.Code, createUsage.Body.String())
		}
	}

	list := requestJSON(t, router, http.MethodGet, "/v1/usages?meter=api_calls&from=2026-06-08T00:00:00Z&to=2026-06-09T00:00:00Z&bucket_size=day", nil)
	if list.Code != http.StatusOK {
		t.Fatalf("list usages status = %d, want %d: %s", list.Code, http.StatusOK, list.Body.String())
	}
	var buckets []usageListItemResponse
	decodeJSON(t, list, &buckets)
	if len(buckets) != 1 || buckets[0].Subject != "" || buckets[0].Quantity != 10 {
		t.Fatalf("all subject buckets = %#v, want one unscoped bucket with quantity 10", buckets)
	}

	search := requestJSON(t, router, http.MethodPost, "/v1/usages/search", map[string]any{
		"meter":       "api_calls",
		"from":        "2026-06-08T00:00:00Z",
		"to":          "2026-06-09T00:00:00Z",
		"bucket_size": "day",
		"group_by":    []string{"subject"},
	})
	if search.Code != http.StatusOK {
		t.Fatalf("search usages status = %d, want %d: %s", search.Code, http.StatusOK, search.Body.String())
	}
	var groupedBuckets []usageListItemResponse
	decodeJSON(t, search, &groupedBuckets)
	if len(groupedBuckets) != 2 {
		t.Fatalf("grouped bucket count = %d, want 2: %#v", len(groupedBuckets), groupedBuckets)
	}
	if groupedBuckets[0].Subject != "org_123" || groupedBuckets[0].Group["subject"] != "org_123" || groupedBuckets[0].Quantity != 5 {
		t.Fatalf("first grouped subject bucket = %#v", groupedBuckets[0])
	}
	if groupedBuckets[1].Subject != "org_456" || groupedBuckets[1].Group["subject"] != "org_456" || groupedBuckets[1].Quantity != 5 {
		t.Fatalf("second grouped subject bucket = %#v", groupedBuckets[1])
	}

	export := requestJSON(t, router, http.MethodGet, "/v1/usages/export?meter=api_calls&from=2026-06-08T00:00:00Z&to=2026-06-09T00:00:00Z&bucket_size=day&group_by=subject", nil)
	if export.Code != http.StatusOK {
		t.Fatalf("export status = %d, want %d: %s", export.Code, http.StatusOK, export.Body.String())
	}

	want := "bucket_start,subject,meter,bucket_size,aggregation,unit,quantity\n2026-06-08T00:00:00Z,org_123,api_calls,day,sum,call,5\n2026-06-08T00:00:00Z,org_456,api_calls,day,sum,call,5\n"
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

func TestUsageAPIAcceptsExtraMetadataOutsideMeterDimensions(t *testing.T) {
	router := newTestRouter()

	createMeter := requestJSON(t, router, http.MethodPost, "/v1/meters", map[string]any{
		"name":        "api_calls",
		"description": "API calls",
		"unit":        "call",
		"aggregation": "sum",
		"dimensions":  meterDimensionsFromSchema(map[string]string{"region": "string"}),
	})
	if createMeter.Code != http.StatusCreated {
		t.Fatalf("create meter status = %d: %s", createMeter.Code, createMeter.Body.String())
	}

	res := requestJSON(t, router, http.MethodPost, "/v1/usages", map[string]any{
		"subject":  "org_123",
		"meter":    "api_calls",
		"quantity": 1,
		"metadata": map[string]any{
			"region": "us-east-1",
			"regoin": "us-east-1",
		},
	})
	if res.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d: %s", res.Code, http.StatusCreated, res.Body.String())
	}

	var usage usageResponse
	decodeJSON(t, res, &usage)
	if usage.Metadata["regoin"] != "us-east-1" {
		t.Fatalf("usage metadata = %#v", usage.Metadata)
	}
}

func TestUsageAPIRejectsInvalidDimensions(t *testing.T) {
	router := newTestRouter()

	createMeter := requestJSON(t, router, http.MethodPost, "/v1/meters", map[string]any{
		"name":        "api_calls",
		"description": "API calls",
		"unit":        "call",
		"aggregation": "sum",
		"dimensions":  meterDimensionsFromSchema(map[string]string{"region": "string"}),
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

func TestMeterAPIRejectsUnsafeDimensionUpdateWithUsage(t *testing.T) {
	router := newTestRouter()

	createMeter := requestJSON(t, router, http.MethodPost, "/v1/meters", map[string]any{
		"name":        "api_calls",
		"description": "API calls",
		"unit":        "call",
		"aggregation": "sum",
		"dimensions": []map[string]any{
			{
				"name":     "region",
				"type":     "string",
				"required": true,
			},
			{
				"name":     "status",
				"type":     "number",
				"required": false,
			},
		},
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
		"metadata": map[string]any{
			"region": "us-east-1",
			"status": 200,
		},
	})
	if createUsage.Code != http.StatusCreated {
		t.Fatalf("create usage status = %d: %s", createUsage.Code, createUsage.Body.String())
	}

	update := requestJSON(t, router, http.MethodPut, "/v1/meters/"+created.ID, map[string]any{
		"dimensions": []map[string]any{
			{
				"name":     "region",
				"type":     "string",
				"required": true,
			},
		},
	})
	if update.Code != http.StatusConflict {
		t.Fatalf("update meter status = %d, want %d: %s", update.Code, http.StatusConflict, update.Body.String())
	}

	var errRes errorResponse
	decodeJSON(t, update, &errRes)
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

func TestUsageExportJobAPIContract(t *testing.T) {
	router := newTestRouter()

	createMeter := requestJSON(t, router, http.MethodPost, "/v1/meters", map[string]any{
		"name":        "exported_events",
		"description": "Exported events",
		"unit":        "event",
		"aggregation": "sum",
		"dimensions": []map[string]any{
			{"name": "region", "type": "string"},
		},
	})
	if createMeter.Code != http.StatusCreated {
		t.Fatalf("create meter status = %d: %s", createMeter.Code, createMeter.Body.String())
	}

	create := requestJSON(t, router, http.MethodPost, "/v1/exports", map[string]any{
		"kind":   "usage_buckets",
		"format": "csv",
		"query": map[string]any{
			"meter":       "exported_events",
			"from":        "2026-06-01T00:00:00Z",
			"to":          "2026-06-02T00:00:00Z",
			"bucket_size": "day",
			"group_by":    []string{"region"},
			"limit":       500,
			"filter": map[string]any{
				"type":  "condition",
				"field": "quantity",
				"op":    "gte",
				"value": 1,
			},
		},
	})
	if create.Code != http.StatusAccepted {
		t.Fatalf("create export job status = %d, want %d: %s", create.Code, http.StatusAccepted, create.Body.String())
	}

	var created exportJobResponse
	decodeJSON(t, create, &created)
	if created.ID == "" || created.Kind != "usage_buckets" || created.Status != "queued" || created.Format != "csv" || created.CreatedAt == "" || created.UpdatedAt == "" || created.CompletedAt != "" {
		t.Fatalf("created export job = %#v", created)
	}
	if created.Query["meter"] != "exported_events" || created.Query["bucket_size"] != "day" {
		t.Fatalf("created export job query = %#v", created.Query)
	}

	get := requestJSON(t, router, http.MethodGet, "/v1/exports/"+created.ID, nil)
	if get.Code != http.StatusOK {
		t.Fatalf("get export job status = %d, want %d: %s", get.Code, http.StatusOK, get.Body.String())
	}
	var found exportJobResponse
	decodeJSON(t, get, &found)
	if found.ID != created.ID || found.Status != "queued" {
		t.Fatalf("found export job = %#v", found)
	}

	list := requestJSON(t, router, http.MethodGet, "/v1/exports?limit=10", nil)
	if list.Code != http.StatusOK {
		t.Fatalf("list export jobs status = %d, want %d: %s", list.Code, http.StatusOK, list.Body.String())
	}
	var jobs exportJobListResponse
	decodeJSON(t, list, &jobs)
	if len(jobs.Items) != 1 || jobs.Items[0].ID != created.ID {
		t.Fatalf("export jobs = %#v", jobs)
	}

	invalid := requestJSON(t, router, http.MethodPost, "/v1/exports", map[string]any{
		"query": map[string]any{
			"meter": "exported_events",
			"from":  "2026-06-01T00:00:00Z",
		},
	})
	if invalid.Code != http.StatusBadRequest {
		t.Fatalf("invalid export job status = %d, want %d: %s", invalid.Code, http.StatusBadRequest, invalid.Body.String())
	}

	missing := requestJSON(t, router, http.MethodGet, "/v1/exports/missing", nil)
	if missing.Code != http.StatusNotFound {
		t.Fatalf("missing export job status = %d, want %d: %s", missing.Code, http.StatusNotFound, missing.Body.String())
	}
}

func newTestRouter() http.Handler {
	store, err := sqlite.NewStore(context.Background(), ":memory:", config.DBPoolConfig{MaxOpenConns: 1})
	if err != nil {
		panic(err)
	}
	if _, err := sqlite.NewAuthRepository(store).SaveWorkspace(context.Background(), appauth.Workspace{
		ID:        appauth.DefaultWorkspaceID,
		Name:      "Default",
		CreatedAt: time.Now().UTC(),
	}); err != nil {
		panic(err)
	}
	meterRepo := sqlite.NewMeterRepository(store)
	usageRepo := sqlite.NewUsageRepository(store)
	authRepo := sqlite.NewAuthRepository(store)
	authService := appauth.NewService(authRepo)
	meterService := appmeter.NewService(meterRepo, usageRepo)
	subjectService := appsubject.NewService(usageRepo)
	entitlementService := appentitlement.NewService(sqlite.NewEntitlementRepository(store), meterRepo, usageRepo, store)
	usageService := appusage.NewService(meterRepo, usageRepo, store)
	systemService := appsystem.NewService(sqlite.NewSystemRepository(store))

	router := chi.NewRouter()
	router.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := appauth.WithWorkspaceID(r.Context(), appauth.DefaultWorkspaceID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	})
	router.Route("/v1", func(r chi.Router) {
		httpauth.NewHandler(authService).RegisterRoutes(r)
		httpmeter.NewHandler(meterService).RegisterRoutes(r, nil)
		httpsubject.NewHandler(subjectService).RegisterRoutes(r, nil)
		httpentitlement.NewHandler(entitlementService).RegisterRoutes(r, nil)
		httpusage.NewHandler(usageService, httpusage.HandlerOptions{Entitlements: entitlementService}).RegisterRoutes(r, nil)
		httpsystem.NewHandler(systemService).RegisterRoutes(r, nil)
	})

	return router
}

func newProtectedTestRouter() http.Handler {
	store, err := sqlite.NewStore(context.Background(), ":memory:", config.DBPoolConfig{MaxOpenConns: 1})
	if err != nil {
		panic(err)
	}
	meterRepo := sqlite.NewMeterRepository(store)
	usageRepo := sqlite.NewUsageRepository(store)
	authRepo := sqlite.NewAuthRepository(store)
	authService := appauth.NewService(authRepo)
	authorizer, err := appauth.NewCasbinAuthorizer()
	if err != nil {
		panic(err)
	}
	meterService := appmeter.NewService(meterRepo, usageRepo)
	subjectService := appsubject.NewService(usageRepo)
	entitlementService := appentitlement.NewService(sqlite.NewEntitlementRepository(store), meterRepo, usageRepo, store)
	usageService := appusage.NewService(meterRepo, usageRepo, store)
	systemService := appsystem.NewService(sqlite.NewSystemRepository(store))
	authHandler := httpauth.NewHandler(authService)

	router := chi.NewRouter()
	router.Route("/v1", func(r chi.Router) {
		authHandler.RegisterRoutes(r)
		r.Group(func(protected chi.Router) {
			protected.Use(authHandler.RequireAuth)
			httpmeter.NewHandler(meterService).RegisterRoutes(protected, authorizer)
			httpsubject.NewHandler(subjectService).RegisterRoutes(protected, authorizer)
			httpentitlement.NewHandler(entitlementService).RegisterRoutes(protected, authorizer)
			httpusage.NewHandler(usageService, httpusage.HandlerOptions{Entitlements: entitlementService}).RegisterRoutes(protected, authorizer)
			httpsystem.NewHandler(systemService).RegisterRoutes(protected, authorizer)
		})
	})

	return router
}

func registerAndLogin(t *testing.T, router http.Handler, email string, password string) []*http.Cookie {
	t.Helper()

	create := requestJSON(t, router, http.MethodPost, "/v1/auth/users", map[string]any{
		"email":    email,
		"password": password,
	})
	if create.Code != http.StatusCreated {
		t.Fatalf("create user %s status = %d, want %d: %s", email, create.Code, http.StatusCreated, create.Body.String())
	}

	login := requestJSON(t, router, http.MethodPost, "/v1/auth/sessions", map[string]any{
		"email":    email,
		"password": password,
	})
	if login.Code != http.StatusCreated {
		t.Fatalf("login user %s status = %d, want %d: %s", email, login.Code, http.StatusCreated, login.Body.String())
	}
	return login.Result().Cookies()
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
	ID                 string                   `json:"id"`
	Name               string                   `json:"name"`
	Description        string                   `json:"description"`
	Unit               string                   `json:"unit"`
	Aggregation        string                   `json:"aggregation"`
	Dimensions         []meterDimensionResponse `json:"dimensions"`
	EventRetentionDays int                      `json:"event_retention_days"`
	CreatedAt          string                   `json:"created_at"`
}

type meterDimensionResponse struct {
	Name        string `json:"name"`
	DisplayName string `json:"display_name"`
	Description string `json:"description"`
	Type        string `json:"type"`
	Required    bool   `json:"required"`
	Deprecated  bool   `json:"deprecated"`
}

func meterDimensionsFromSchema(schema map[string]string) []map[string]any {
	dimensions := make([]map[string]any, 0, len(schema))
	names := make([]string, 0, len(schema))
	for name := range schema {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		dimensions = append(dimensions, map[string]any{
			"name":     name,
			"type":     schema[name],
			"required": true,
		})
	}
	return dimensions
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

type entitlementPlanResponse struct {
	ID     string                     `json:"id"`
	Name   string                     `json:"name"`
	Limits []entitlementLimitResponse `json:"limits"`
}

type entitlementLimitResponse struct {
	Meter          string  `json:"meter"`
	Period         string  `json:"period"`
	Limit          float64 `json:"limit"`
	WarningPercent float64 `json:"warning_percent"`
}

type entitlementCheckResponse struct {
	Allowed           bool    `json:"allowed"`
	State             string  `json:"state"`
	Subject           string  `json:"subject"`
	Meter             string  `json:"meter"`
	Quantity          float64 `json:"quantity"`
	Current           float64 `json:"current"`
	Limit             float64 `json:"limit"`
	Remaining         float64 `json:"remaining"`
	Overage           float64 `json:"overage"`
	PeriodResetAt     string  `json:"period_reset_at"`
	RetryAfterSeconds int64   `json:"retry_after_seconds"`
	Message           string  `json:"message"`
}

type entitlementProgressResponse struct {
	Subject string                         `json:"subject"`
	Items   []entitlementProgressItemReply `json:"items"`
}

type entitlementProgressItemReply struct {
	Meter         string  `json:"meter"`
	Period        string  `json:"period"`
	State         string  `json:"state"`
	Current       float64 `json:"current"`
	Limit         float64 `json:"limit"`
	Remaining     float64 `json:"remaining"`
	Overage       float64 `json:"overage"`
	PeriodResetAt string  `json:"period_reset_at"`
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

type dimensionValueResponse struct {
	Field       string `json:"field"`
	Value       string `json:"value"`
	UsageEvents int    `json:"events"`
}

type dimensionValueListResponse struct {
	Items []dimensionValueResponse `json:"items"`
}

type breakdownResponse struct {
	Field       string  `json:"field"`
	Value       string  `json:"value"`
	Quantity    float64 `json:"quantity"`
	UsageEvents int     `json:"events"`
	Aggregation string  `json:"aggregation"`
	Unit        string  `json:"unit"`
}

type breakdownListResponse struct {
	Items []breakdownResponse `json:"items"`
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

type exportJobListResponse struct {
	Items      []exportJobResponse `json:"items"`
	NextCursor string              `json:"next_cursor"`
}

type exportJobResponse struct {
	ID          string         `json:"id"`
	Kind        string         `json:"kind"`
	Status      string         `json:"status"`
	Format      string         `json:"format"`
	Query       map[string]any `json:"query"`
	Error       string         `json:"error"`
	CreatedAt   string         `json:"created_at"`
	UpdatedAt   string         `json:"updated_at"`
	CompletedAt string         `json:"completed_at"`
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

type authAPIKeyCreateResponse struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Prefix     string `json:"prefix"`
	Key        string `json:"key"`
	CreatedAt  string `json:"created_at"`
	LastUsedAt string `json:"last_used_at"`
}

type authAPIKeyListResponse struct {
	Items []authAPIKeyCreateResponse `json:"items"`
}
