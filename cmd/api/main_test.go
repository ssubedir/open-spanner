package main

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestReadyReturnsNoContentWhenStorageIsReady(t *testing.T) {
	res := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/ready", nil)

	ready(fakeReadyChecker{})(res, req)

	if res.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusNoContent)
	}
}

func TestReadyReturnsServiceUnavailableWhenStorageFails(t *testing.T) {
	res := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/ready", nil)

	ready(fakeReadyChecker{err: errors.New("storage unavailable")})(res, req)

	if res.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusServiceUnavailable)
	}
}

type fakeReadyChecker struct {
	err error
}

func (c fakeReadyChecker) Ready(ctx context.Context) error {
	return c.err
}
