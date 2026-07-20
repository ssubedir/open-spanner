package history

import (
	"context"
	"errors"
	"testing"
	"time"

	appsystem "github.com/ssubedir/open-spanner/internal/metering/app/system"
)

type serviceStub struct {
	before time.Time
	batch  int
	result appsystem.OperationalHistoryPruneResult
	err    error
}

func (s *serviceStub) PruneOperationalHistory(_ context.Context, before time.Time, batch int) (appsystem.OperationalHistoryPruneResult, error) {
	s.before, s.batch = before, batch
	return s.result, s.err
}

type metricStub struct{ rows, failures int }

func (m *metricStub) RecordOperationalHistoryCleanup(_ context.Context, rows int) { m.rows += rows }
func (m *metricStub) RecordOperationalHistoryCleanupFailure(context.Context)      { m.failures++ }

func TestProcessOnceRecordsCleanup(t *testing.T) {
	service := &serviceStub{result: appsystem.OperationalHistoryPruneResult{IngestionAudits: 3, ExportJobs: 2}}
	metrics := &metricStub{}
	worker := NewWorker(service, 24*time.Hour, time.Hour, time.Second, 100, metrics, func(string, ...any) {})
	result, err := worker.ProcessOnce(context.Background())
	if err != nil {
		t.Fatalf("process once: %v", err)
	}
	if result.Total() != 5 || metrics.rows != 5 || metrics.failures != 0 || service.batch != 100 {
		t.Fatalf("result=%+v metrics=%+v batch=%d", result, metrics, service.batch)
	}
	if age := time.Since(service.before); age < 23*time.Hour || age > 25*time.Hour {
		t.Fatalf("cutoff age=%s", age)
	}
}

func TestProcessOnceRecordsFailure(t *testing.T) {
	metrics := &metricStub{}
	worker := NewWorker(&serviceStub{err: errors.New("cleanup failed")}, time.Hour, time.Hour, time.Second, 10, metrics, nil)
	if _, err := worker.ProcessOnce(context.Background()); err == nil || metrics.failures != 1 {
		t.Fatalf("error=%v failures=%d", err, metrics.failures)
	}
}
