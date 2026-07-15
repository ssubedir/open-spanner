package export

import (
	"context"
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
