package heartbeat

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

func TestStartRecordsImmediatelyAndStopsIdempotently(t *testing.T) {
	t.Setenv("OPEN_SPANNER_WORKER_INSTANCE_ID", "export-pod-2")
	recorder := &testRecorder{}
	stop := Start(context.Background(), recorder, "export", func(string, ...any) {})
	if recorder.calls.Load() != 1 {
		t.Fatalf("heartbeat calls = %d, want immediate record", recorder.calls.Load())
	}
	if recorder.instanceID != "export-pod-2" {
		t.Fatalf("heartbeat instance id = %q, want configured pod name", recorder.instanceID)
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
	if recorder.removals.Load() != 1 {
		t.Fatalf("heartbeat removals = %d, want one", recorder.removals.Load())
	}
}

type testRecorder struct {
	calls      atomic.Int32
	removals   atomic.Int32
	instanceID string
}

func (r *testRecorder) RemoveWorkerHeartbeat(_ context.Context, _, instanceID string) error {
	r.instanceID = instanceID
	r.removals.Add(1)
	return nil
}

func (r *testRecorder) RecordWorkerHeartbeat(_ context.Context, _, instanceID string, _, _ time.Time) error {
	r.instanceID = instanceID
	r.calls.Add(1)
	return nil
}
