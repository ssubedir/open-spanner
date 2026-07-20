package http

import (
	"context"
	stdhttp "net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestStopDrainsInflightRequestBeforeCleanup(t *testing.T) {
	requestStarted := make(chan struct{})
	releaseRequest := make(chan struct{})
	handler := stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, _ *stdhttp.Request) {
		close(requestStarted)
		<-releaseRequest
		w.WriteHeader(stdhttp.StatusNoContent)
	})
	testServer := httptest.NewUnstartedServer(handler)
	testServer.Start()
	t.Cleanup(testServer.Close)

	draining := make(chan struct{})
	cleaned := make(chan struct{})
	server := &Server{
		httpServer: testServer.Config,
		beginDrain: func() { close(draining) },
		cleanup:    func() error { close(cleaned); return nil },
	}
	requestErr := make(chan error, 1)
	go func() {
		response, err := testServer.Client().Get(testServer.URL)
		if err == nil {
			response.Body.Close()
		}
		requestErr <- err
	}()
	<-requestStarted

	stopErr := make(chan error, 1)
	go func() { stopErr <- server.stop(nil) }()
	select {
	case <-draining:
	case <-time.After(time.Second):
		t.Fatal("server did not enter draining state")
	}
	select {
	case <-cleaned:
		t.Fatal("resources cleaned before in-flight request completed")
	default:
	}

	close(releaseRequest)
	if err := <-requestErr; err != nil {
		t.Fatalf("in-flight request: %v", err)
	}
	if err := <-stopErr; err != nil {
		t.Fatalf("stop server: %v", err)
	}
	select {
	case <-cleaned:
	default:
		t.Fatal("resources were not cleaned after request drained")
	}
}

func TestCleanupResourcesRunsOnce(t *testing.T) {
	calls := 0
	server := &Server{cleanup: func() error { calls++; return context.Canceled }}
	for range 2 {
		if err := server.cleanupResources(); err != context.Canceled {
			t.Fatalf("cleanup error = %v", err)
		}
	}
	if calls != 1 {
		t.Fatalf("cleanup calls = %d, want 1", calls)
	}
}
