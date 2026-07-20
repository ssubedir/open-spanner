package export

import (
	"context"
	"errors"
	"io"
	"sync"
	"testing"
	"time"

	"github.com/ssubedir/open-spanner/internal/metering/adapters/fileexport"
	appusage "github.com/ssubedir/open-spanner/internal/metering/app/usage"
	"github.com/ssubedir/open-spanner/internal/metering/domain"
)

const testExportQuery = `{"meter":"requests","from":"2026-01-01T00:00:00Z","to":"2026-01-02T00:00:00Z","bucket_size":"day","limit":10}`

type leaseTestService struct {
	mu             sync.Mutex
	renewals       int
	failRenewal    bool
	completedToken string
	failed         bool
}

func (s *leaseTestService) ClaimExportJob(context.Context, appusage.ExportJobClaimCommand) (appusage.ExportJobResult, bool, error) {
	return appusage.ExportJobResult{ID: "job-1", WorkspaceID: "workspace-1", QueryJSON: testExportQuery, ClaimToken: "11111111-1111-4111-8111-111111111111"}, true, nil
}

func (s *leaseTestService) RenewExportJobLease(context.Context, appusage.ExportJobRenewCommand) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.renewals++
	if s.failRenewal {
		return domain.ErrNotFound
	}
	return nil
}

func (s *leaseTestService) CompleteExportJob(_ context.Context, cmd appusage.ExportJobCompleteCommand) (appusage.ExportJobResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.completedToken = cmd.ClaimToken
	return appusage.ExportJobResult{}, nil
}

func (s *leaseTestService) FailExportJob(context.Context, appusage.ExportJobFailCommand) (appusage.ExportJobResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.failed = true
	return appusage.ExportJobResult{}, nil
}

func (s *leaseTestService) List(ctx context.Context, _ appusage.ListQuery) ([]appusage.ListItemResult, error) {
	for {
		s.mu.Lock()
		renewals, failRenewal := s.renewals, s.failRenewal
		s.mu.Unlock()
		if !failRenewal && renewals >= 3 {
			return nil, nil
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(time.Millisecond):
		}
	}
}

func TestWorkerRenewsLeaseAndCompletesWithClaimToken(t *testing.T) {
	service := &leaseTestService{}
	worker := NewWorker(service, fileexport.NewStore(t.TempDir()), time.Millisecond, 15*time.Millisecond, 3, func(string, ...any) {})
	processed, err := worker.ProcessOnce(context.Background())
	if err != nil || !processed {
		t.Fatalf("ProcessOnce() processed=%v err=%v", processed, err)
	}
	service.mu.Lock()
	defer service.mu.Unlock()
	if service.renewals < 3 {
		t.Fatalf("renewals=%d, want at least 3", service.renewals)
	}
	if service.completedToken != "11111111-1111-4111-8111-111111111111" {
		t.Fatalf("completed token=%q", service.completedToken)
	}
	if service.failed {
		t.Fatal("completed job was also failed")
	}
}

func TestWorkerStopsWhenLeaseIsLost(t *testing.T) {
	service := &leaseTestService{failRenewal: true}
	worker := NewWorker(service, fileexport.NewStore(t.TempDir()), time.Millisecond, 9*time.Millisecond, 3, func(string, ...any) {})
	processed, err := worker.ProcessOnce(context.Background())
	if err != nil || !processed {
		t.Fatalf("ProcessOnce() processed=%v err=%v", processed, err)
	}
	service.mu.Lock()
	defer service.mu.Unlock()
	if service.completedToken != "" {
		t.Fatalf("lost lease completed with token %q", service.completedToken)
	}
	if service.failed {
		t.Fatal("lost lease was failed by stale owner")
	}
	if service.renewals != 1 {
		t.Fatalf("renewals=%d, want 1", service.renewals)
	}
}

var _ Service = (*leaseTestService)(nil)

type cleanupTestService struct {
	leaseTestService
	jobs []appusage.ExportJobResult
	mark bool
	runs []appusage.ExportCleanupRunCommand
}

func (s *cleanupTestService) ListExpiredExportJobs(context.Context, time.Time, int) ([]appusage.ExportJobResult, error) {
	return s.jobs, nil
}

func (s *cleanupTestService) ExpireExportJob(context.Context, string) (bool, error) {
	return s.mark, nil
}

func (s *cleanupTestService) RecordExportCleanupRun(_ context.Context, cmd appusage.ExportCleanupRunCommand) (appusage.ExportCleanupRunResult, error) {
	s.runs = append(s.runs, cmd)
	return appusage.ExportCleanupRunResult{}, nil
}

func TestCleanupRemovesArtifactMarksJobAndRecordsMetrics(t *testing.T) {
	store := fileexport.NewStore(t.TempDir())
	artifact, err := store.Write(context.Background(), "job-1.csv", func(writer io.Writer) error {
		_, err := io.WriteString(writer, "some export data")
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	service := &cleanupTestService{jobs: []appusage.ExportJobResult{{ID: "job-1", WorkspaceID: "workspace-1", ArtifactPath: artifact.Name, ArtifactSize: artifact.Size}}, mark: true}
	worker := NewWorker(service, store, time.Second, time.Minute, 3, func(string, ...any) {}).WithCleanup(time.Hour, time.Hour, 10)
	expired, err := worker.CleanupOnce(context.Background())
	if err != nil || expired != 1 {
		t.Fatalf("CleanupOnce() expired=%d err=%v", expired, err)
	}
	if _, err := store.Open(context.Background(), artifact.Name); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("artifact open error=%v, want not exist", err)
	}
	if len(service.runs) != 1 || service.runs[0].FilesDeleted != 1 || service.runs[0].BytesReclaimed != artifact.Size || service.runs[0].Failures != 0 {
		t.Fatalf("cleanup runs=%#v", service.runs)
	}
}

func TestCleanupDoesNotDoubleCountConcurrentlyExpiredJob(t *testing.T) {
	store := fileexport.NewStore(t.TempDir())
	service := &cleanupTestService{jobs: []appusage.ExportJobResult{{ID: "job-1", WorkspaceID: "workspace-1", ArtifactPath: "missing.csv", ArtifactSize: 42}}, mark: false}
	worker := NewWorker(service, store, time.Second, time.Minute, 3, func(string, ...any) {}).WithCleanup(time.Hour, time.Hour, 10)
	expired, err := worker.CleanupOnce(context.Background())
	if err != nil || expired != 0 {
		t.Fatalf("CleanupOnce() expired=%d err=%v", expired, err)
	}
	if len(service.runs) != 1 || service.runs[0].FilesDeleted != 0 || service.runs[0].BytesReclaimed != 0 || service.runs[0].Failures != 0 {
		t.Fatalf("cleanup runs=%#v", service.runs)
	}
}

var _ CleanupService = (*cleanupTestService)(nil)
