package main

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestReadyReturnsNoContentWhenStorageIsReady(t *testing.T) {
	res := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/ready", nil)

	ready(fakeReadyChecker{}, &drainState{})(res, req)

	if res.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusNoContent)
	}
}

func TestReadyReturnsServiceUnavailableWhenStorageFails(t *testing.T) {
	res := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/ready", nil)

	ready(fakeReadyChecker{err: errors.New("storage unavailable")}, &drainState{})(res, req)

	if res.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusServiceUnavailable)
	}
}

func TestReadyReturnsServiceUnavailableWhileDraining(t *testing.T) {
	state := &drainState{}
	state.Begin()
	checker := &countingReadyChecker{}
	res := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/ready", nil)

	ready(checker, state)(res, req)

	if res.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusServiceUnavailable)
	}
	if checker.calls.Load() != 0 {
		t.Fatalf("storage readiness called %d times while draining", checker.calls.Load())
	}
}

func TestGRPCDrainerWaitsForGracefulStop(t *testing.T) {
	server := newFakeGRPCStopper()
	drainer := newGRPCDrainer(server)
	drainer.Begin()
	<-server.started
	close(server.release)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if drainer.Wait(ctx) {
		t.Fatal("graceful shutdown was forced")
	}
	if server.forced.Load() {
		t.Fatal("grpc Stop was called after graceful completion")
	}
}

func TestGRPCDrainerForcesStopAfterDeadline(t *testing.T) {
	server := newFakeGRPCStopper()
	drainer := newGRPCDrainer(server)
	drainer.Begin()
	<-server.started
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if !drainer.Wait(ctx) {
		t.Fatal("blocked graceful shutdown was not forced")
	}
	if !server.forced.Load() {
		t.Fatal("grpc Stop was not called")
	}
}

func TestStopFunctionsHonorsDeadline(t *testing.T) {
	release := make(chan struct{})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := stopFunctions(ctx, func() { <-release }); !errors.Is(err, context.Canceled) {
		t.Fatalf("stop functions error = %v", err)
	}
	close(release)
}

type fakeReadyChecker struct {
	err error
}

func (c fakeReadyChecker) Ready(ctx context.Context) error {
	return c.err
}

type countingReadyChecker struct {
	calls atomic.Int32
}

func (c *countingReadyChecker) Ready(context.Context) error {
	c.calls.Add(1)
	return nil
}

type fakeGRPCStopper struct {
	started chan struct{}
	release chan struct{}
	forced  atomic.Bool
	once    sync.Once
}

func newFakeGRPCStopper() *fakeGRPCStopper {
	return &fakeGRPCStopper{started: make(chan struct{}), release: make(chan struct{})}
}

func (s *fakeGRPCStopper) GracefulStop() {
	close(s.started)
	<-s.release
}

func (s *fakeGRPCStopper) Stop() {
	s.forced.Store(true)
	s.once.Do(func() { close(s.release) })
}
