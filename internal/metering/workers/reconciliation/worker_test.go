package reconciliation

import (
	"context"
	"errors"
	"testing"
	"time"

	appsystem "github.com/ssubedir/open-spanner/internal/metering/app/system"
)

type fakeService struct {
	claim           appsystem.ReconciliationClaim
	hasClaim        bool
	run             appsystem.ReconciliationRun
	notify          bool
	runErr          error
	failed          bool
	notified        bool
	markedValue     string
	notification    appsystem.ReconciliationNotification
	hasNotification bool
	completed       bool
	retried         bool
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
func (f *fakeService) ClaimReconciliationNotification(context.Context, time.Time, time.Time) (appsystem.ReconciliationNotification, bool, error) {
	return f.notification, f.hasNotification, nil
}
func (f *fakeService) CompleteReconciliationNotification(context.Context, appsystem.ReconciliationNotification) error {
	f.completed = true
	return nil
}
func (f *fakeService) RetryReconciliationNotification(context.Context, appsystem.ReconciliationNotification, time.Time, int, error) error {
	f.retried = true
	return nil
}

type fakeNotifier struct {
	calls int
	err   error
}

func (n *fakeNotifier) Notify(context.Context, appsystem.ReconciliationNotification) error {
	n.calls++
	return n.err
}

func TestProcessOnceDeliversPendingNotification(t *testing.T) {
	service := &fakeService{notification: appsystem.ReconciliationNotification{ID: "notification", WorkspaceID: "workspace", EventType: "drift_detected"}, hasNotification: true}
	notifier := &fakeNotifier{}
	worker := NewWorker(service, Options{LockTTL: time.Minute, ScheduleInterval: time.Hour, RetryAfter: time.Minute, MaxAttempts: 5, Notifier: notifier, Logger: func(string, ...any) {}})
	processed, err := worker.ProcessOnce(context.Background())
	if err != nil || !processed || notifier.calls != 1 || !service.completed {
		t.Fatalf("processed=%v err=%v calls=%d completed=%v", processed, err, notifier.calls, service.completed)
	}
}

func TestProcessOnceRetriesFailedNotification(t *testing.T) {
	service := &fakeService{notification: appsystem.ReconciliationNotification{ID: "notification", WorkspaceID: "workspace", EventType: "drift_detected"}, hasNotification: true}
	notifier := &fakeNotifier{err: errors.New("unavailable")}
	worker := NewWorker(service, Options{LockTTL: time.Minute, ScheduleInterval: time.Hour, RetryAfter: time.Minute, MaxAttempts: 5, Notifier: notifier, Logger: func(string, ...any) {}})
	processed, err := worker.ProcessOnce(context.Background())
	if !processed || err != nil || !service.retried || service.completed {
		t.Fatalf("processed=%v err=%v retried=%v completed=%v", processed, err, service.retried, service.completed)
	}
}

func TestDeliveryRetryDelayBacksOffAndCaps(t *testing.T) {
	if got := deliveryRetryDelay(time.Minute, 3); got != 8*time.Minute {
		t.Fatalf("delay = %s, want 8m", got)
	}
	if got := deliveryRetryDelay(time.Minute, 20); got != time.Hour {
		t.Fatalf("capped delay = %s, want 1h", got)
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
