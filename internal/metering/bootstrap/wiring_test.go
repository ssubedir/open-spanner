package bootstrap

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/ssubedir/open-spanner/internal/config"
)

func TestRegisterRoutesRequiresAPIKeyForSDKClients(t *testing.T) {
	ctx := context.Background()
	router := chi.NewRouter()
	app, err := RegisterRoutes(ctx, router, config.Config{
		DBDriver:   "sqlite",
		SQLitePath: ":memory:",
		DBPool:     config.DBPoolConfig{MaxOpenConns: 1},
	})
	if err != nil {
		t.Fatalf("register routes: %v", err)
	}
	t.Cleanup(func() {
		if err := app.Cleanup(); err != nil {
			t.Fatalf("cleanup: %v", err)
		}
	})

	req := httptest.NewRequest(http.MethodGet, "/v1/meters", nil)
	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)
	if res.Code != http.StatusUnauthorized {
		t.Fatalf("meters status = %d, want %d: %s", res.Code, http.StatusUnauthorized, res.Body.String())
	}

	createUser := requestJSON(t, router, http.MethodPost, "/v1/auth/users", map[string]any{
		"email":    "admin@example.com",
		"password": "strong-password",
	}, nil)
	if createUser.Code != http.StatusCreated {
		t.Fatalf("create user status = %d, want %d: %s", createUser.Code, http.StatusCreated, createUser.Body.String())
	}

	login := requestJSON(t, router, http.MethodPost, "/v1/auth/sessions", map[string]any{
		"email":    "admin@example.com",
		"password": "strong-password",
	}, nil)
	if login.Code != http.StatusCreated {
		t.Fatalf("login status = %d, want %d: %s", login.Code, http.StatusCreated, login.Body.String())
	}

	keyRes := requestJSON(t, router, http.MethodPost, "/v1/auth/api-keys", map[string]any{
		"name": "sdk",
	}, login.Result().Cookies())
	if keyRes.Code != http.StatusCreated {
		t.Fatalf("create api key status = %d, want %d: %s", keyRes.Code, http.StatusCreated, keyRes.Body.String())
	}

	var key struct {
		Key string `json:"key"`
	}
	if err := json.NewDecoder(keyRes.Body).Decode(&key); err != nil {
		t.Fatalf("decode api key: %v", err)
	}
	if key.Key == "" {
		t.Fatal("api key is empty")
	}

	req = httptest.NewRequest(http.MethodGet, "/v1/meters", nil)
	req.Header.Set("Authorization", "Bearer "+key.Key)
	res = httptest.NewRecorder()
	router.ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("api-key meters status = %d, want %d: %s", res.Code, http.StatusOK, res.Body.String())
	}
}

func requestJSON(t *testing.T, handler http.Handler, method string, path string, body any, cookies []*http.Cookie) *httptest.ResponseRecorder {
	t.Helper()

	var payload bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&payload).Encode(body); err != nil {
			t.Fatalf("encode request body: %v", err)
		}
	}

	req := httptest.NewRequest(method, path, &payload)
	req.Header.Set("Content-Type", "application/json")
	for _, cookie := range cookies {
		req.AddCookie(cookie)
	}
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)
	return res
}
