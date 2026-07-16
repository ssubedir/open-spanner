package heartbeat

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

func TestStartRecordsImmediatelyAndStopsIdempotently(t *testing.T) {
	recorder := &testRecorder{}
	stop := Start(context.Background(), recorder, "export", func(string, ...any) {})
	if recorder.calls.Load() != 1 {
		t.Fatalf("heartbeat calls = %d, want immediate record", recorder.calls.Load())
	}

	done := make(chan struct{})
	go func() {
		stop()
		stop()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("heartbeat did not stop")
	}
}

type testRecorder struct {
	calls atomic.Int32
}

func (r *testRecorder) RecordWorkerHeartbeat(context.Context, string, time.Time, time.Time) error {
	r.calls.Add(1)
	return nil
}
