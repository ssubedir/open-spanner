package health

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"
)

type checker struct{ err error }

func (c checker) Ready(context.Context) error { return c.err }

func TestServerHealthAndReadiness(t *testing.T) {
	server, err := Start("127.0.0.1:0", checker{}, nil)
	if err != nil {
		t.Fatalf("start server: %v", err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = server.Shutdown(ctx)
	})

	baseURL := "http://" + server.Addr().String()
	assertStatus(t, baseURL+"/health", http.StatusNoContent)
	assertStatus(t, baseURL+"/ready", http.StatusNoContent)
	server.BeginDrain()
	assertStatus(t, baseURL+"/ready", http.StatusServiceUnavailable)
	assertStatus(t, baseURL+"/health", http.StatusNoContent)
}

func TestServerReadinessFailure(t *testing.T) {
	server, err := Start("127.0.0.1:0", checker{err: errors.New("database unavailable")}, nil)
	if err != nil {
		t.Fatalf("start server: %v", err)
	}
	defer func() { _ = server.Shutdown(context.Background()) }()
	assertStatus(t, "http://"+server.Addr().String()+"/ready", http.StatusServiceUnavailable)
}

func assertStatus(t *testing.T, url string, want int) {
	t.Helper()
	response, err := http.Get(url)
	if err != nil {
		t.Fatalf("get %s: %v", url, err)
	}
	defer response.Body.Close()
	if response.StatusCode != want {
		t.Fatalf("GET %s status=%d, want %d", url, response.StatusCode, want)
	}
}
