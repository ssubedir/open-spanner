package reconciliation

import (
	"context"
	"errors"
	"testing"
	"time"

	appsystem "github.com/ssubedir/open-spanner/internal/metering/app/system"
)

type fakeService struct {
	claim       appsystem.ReconciliationClaim
	hasClaim    bool
	run         appsystem.ReconciliationRun
	notify      bool
	runErr      error
	failed      bool
	notified    bool
	markedValue string
}

func (f *fakeService) ClaimScheduledReconciliation(context.Context, time.Time, time.Time) (appsystem.ReconciliationClaim, bool, error) {
	return f.claim, f.hasClaim, nil
}
func (f *fakeService) RunScheduledReconciliation(context.Context, appsystem.ReconciliationClaim, time.Duration, appsystem.ReconciliationQuery) (appsystem.ReconciliationRun, bool, error) {
	return f.run, f.notify, f.runErr
}
func (f *fakeService) FailScheduledReconciliation(context.Context, appsystem.ReconciliationClaim, time.Time, error) error {
	f.failed = true
	return nil
}
func (f *fakeService) MarkReconciliationNotified(_ context.Context, _ string, fingerprint string) error {
	f.notified, f.markedValue = true, fingerprint
	return nil
}

type fakeNotifier struct {
	calls int
	err   error
}

func (n *fakeNotifier) Notify(context.Context, string, appsystem.ReconciliationRun) error {
	n.calls++
	return n.err
}

func TestProcessOnceNotifiesAndMarksDrift(t *testing.T) {
	service := &fakeService{claim: appsystem.ReconciliationClaim{WorkspaceID: "workspace"}, hasClaim: true, run: appsystem.ReconciliationRun{Status: "drift_detected", Fingerprint: "abc", IssueCount: 1}, notify: true}
	notifier := &fakeNotifier{}
	worker := NewWorker(service, Options{LockTTL: time.Minute, ScheduleInterval: time.Hour, RetryAfter: time.Minute, Limit: 100, LookbackHours: 24, Notifier: notifier, Logger: func(string, ...any) {}})
	processed, err := worker.ProcessOnce(context.Background())
	if err != nil || !processed || notifier.calls != 1 || !service.notified || service.markedValue != "abc" {
		t.Fatalf("processed=%v err=%v calls=%d notified=%v fingerprint=%q", processed, err, notifier.calls, service.notified, service.markedValue)
	}
}

func TestProcessOnceDoesNotMarkFailedNotification(t *testing.T) {
	service := &fakeService{claim: appsystem.ReconciliationClaim{WorkspaceID: "workspace"}, hasClaim: true, run: appsystem.ReconciliationRun{Fingerprint: "abc"}, notify: true}
	notifier := &fakeNotifier{err: errors.New("unavailable")}
	worker := NewWorker(service, Options{LockTTL: time.Minute, ScheduleInterval: time.Hour, RetryAfter: time.Minute, Notifier: notifier, Logger: func(string, ...any) {}})
	processed, err := worker.ProcessOnce(context.Background())
	if !processed || err == nil || service.notified {
		t.Fatalf("processed=%v err=%v notified=%v", processed, err, service.notified)
	}
}

func TestProcessOnceAuditsScanFailure(t *testing.T) {
	service := &fakeService{claim: appsystem.ReconciliationClaim{WorkspaceID: "workspace"}, hasClaim: true, runErr: errors.New("scan failed")}
	worker := NewWorker(service, Options{LockTTL: time.Minute, ScheduleInterval: time.Hour, RetryAfter: time.Minute, Logger: func(string, ...any) {}})
	processed, err := worker.ProcessOnce(context.Background())
	if !processed || err == nil || !service.failed {
		t.Fatalf("processed=%v err=%v failed=%v", processed, err, service.failed)
	}
}
