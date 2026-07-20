package outbox

import (
	"context"
	"errors"
	"testing"
	"time"

	appauth "github.com/ssubedir/open-spanner/internal/auth"
	appalert "github.com/ssubedir/open-spanner/internal/metering/app/alert"
	appentitlement "github.com/ssubedir/open-spanner/internal/metering/app/entitlement"
	appusage "github.com/ssubedir/open-spanner/internal/metering/app/usage"
	domainusage "github.com/ssubedir/open-spanner/internal/metering/domain/usage"
)

type testService struct {
	message   domainusage.OutboxMessage
	available bool
	claimErr  error
	completed []appusage.OutboxCompleteCommand
	failed    []appusage.OutboxFailCommand
}

func (s *testService) ClaimOutbox(context.Context, appusage.OutboxClaimCommand) (domainusage.OutboxMessage, bool, error) {
	return s.message, s.available, s.claimErr
}

func (s *testService) CompleteOutbox(_ context.Context, cmd appusage.OutboxCompleteCommand) error {
	s.completed = append(s.completed, cmd)
	return nil
}

func (s *testService) FailOutbox(_ context.Context, cmd appusage.OutboxFailCommand) error {
	s.failed = append(s.failed, cmd)
	return nil
}

type testAlerts struct {
	events      []appalert.UsageEvent
	workspaceID string
	err         error
}

func (e *testAlerts) EnqueueForUsageEvents(ctx context.Context, events []appalert.UsageEvent) error {
	e.workspaceID, _ = appauth.RequireWorkspaceID(ctx)
	e.events = append(e.events, events...)
	return e.err
}

type testEntitlements struct {
	events      []appentitlement.UsageEvent
	workspaceID string
	err         error
}

func (e *testEntitlements) EnqueueForUsageEvents(ctx context.Context, events []appentitlement.UsageEvent) error {
	e.workspaceID, _ = appauth.RequireWorkspaceID(ctx)
	e.events = append(e.events, events...)
	return e.err
}

func testMessage() domainusage.OutboxMessage {
	return domainusage.OutboxMessage{
		ID: "message-1", WorkspaceID: "workspace-1", EventID: "event-1",
		Subject: "subject-1", MeterName: "requests", Quantity: 7,
		Metadata: map[string]any{"region": "us-east-1"}, Attempts: 1, ClaimToken: "claim-1",
	}
}

func TestProcessOnceFansOutAndCompletes(t *testing.T) {
	service := &testService{message: testMessage(), available: true}
	alerts := &testAlerts{}
	entitlements := &testEntitlements{}
	worker := NewWorker(service, alerts, entitlements, Options{LockTTL: time.Minute, Timeout: time.Second, RetryAfter: time.Second, MaxAttempts: 3})

	processed, err := worker.ProcessOnce(context.Background())
	if err != nil || !processed {
		t.Fatalf("ProcessOnce() processed=%v err=%v", processed, err)
	}
	if len(alerts.events) != 1 || alerts.events[0].Subject != "subject-1" || alerts.events[0].Meter != "requests" {
		t.Fatalf("alert events=%#v", alerts.events)
	}
	if alerts.workspaceID != "workspace-1" {
		t.Fatalf("alert workspace=%q", alerts.workspaceID)
	}
	if len(entitlements.events) != 1 || entitlements.events[0].Quantity != 7 {
		t.Fatalf("entitlement events=%#v", entitlements.events)
	}
	if entitlements.workspaceID != "workspace-1" {
		t.Fatalf("entitlement workspace=%q", entitlements.workspaceID)
	}
	if len(service.completed) != 1 || service.completed[0].ClaimToken != "claim-1" || len(service.failed) != 0 {
		t.Fatalf("completed=%#v failed=%#v", service.completed, service.failed)
	}
}

func TestProcessOnceRetriesPartialFanoutFailure(t *testing.T) {
	service := &testService{message: testMessage(), available: true}
	alerts := &testAlerts{}
	entitlements := &testEntitlements{err: errors.New("entitlement queue unavailable")}
	worker := NewWorker(service, alerts, entitlements, Options{LockTTL: time.Minute, Timeout: time.Second, RetryAfter: 2 * time.Second, MaxAttempts: 4})

	processed, err := worker.ProcessOnce(context.Background())
	if err != nil || !processed {
		t.Fatalf("ProcessOnce() processed=%v err=%v", processed, err)
	}
	if len(alerts.events) != 1 || len(entitlements.events) != 1 {
		t.Fatalf("fanout alerts=%d entitlements=%d", len(alerts.events), len(entitlements.events))
	}
	if len(service.completed) != 0 || len(service.failed) != 1 {
		t.Fatalf("completed=%#v failed=%#v", service.completed, service.failed)
	}
	failure := service.failed[0]
	if failure.ID != "message-1" || failure.ClaimToken != "claim-1" || failure.MaxAttempts != 4 || failure.RetryAfter != 2*time.Second || failure.Error != "entitlement queue unavailable" {
		t.Fatalf("failure=%#v", failure)
	}
}

func TestProcessOnceDoesNotCallEntitlementsWhenAlertFanoutFails(t *testing.T) {
	service := &testService{message: testMessage(), available: true}
	alerts := &testAlerts{err: errors.New("alert queue unavailable")}
	entitlements := &testEntitlements{}
	worker := NewWorker(service, alerts, entitlements, Options{LockTTL: time.Minute, Timeout: time.Second, RetryAfter: time.Second, MaxAttempts: 3})

	processed, err := worker.ProcessOnce(context.Background())
	if err != nil || !processed {
		t.Fatalf("ProcessOnce() processed=%v err=%v", processed, err)
	}
	if len(entitlements.events) != 0 || len(service.failed) != 1 || len(service.completed) != 0 {
		t.Fatalf("entitlements=%#v completed=%#v failed=%#v", entitlements.events, service.completed, service.failed)
	}
}

func TestProcessOnceReturnsIdleAndClaimErrors(t *testing.T) {
	worker := NewWorker(&testService{}, &testAlerts{}, &testEntitlements{}, Options{LockTTL: time.Minute, MaxAttempts: 3})
	processed, err := worker.ProcessOnce(context.Background())
	if err != nil || processed {
		t.Fatalf("idle ProcessOnce() processed=%v err=%v", processed, err)
	}

	wantErr := errors.New("database unavailable")
	worker = NewWorker(&testService{claimErr: wantErr}, &testAlerts{}, &testEntitlements{}, Options{LockTTL: time.Minute, MaxAttempts: 3})
	processed, err = worker.ProcessOnce(context.Background())
	if processed || !errors.Is(err, wantErr) {
		t.Fatalf("failed ProcessOnce() processed=%v err=%v", processed, err)
	}
}

func TestRetryDelayIsExponentialAndCapped(t *testing.T) {
	if got := retryDelay(2*time.Second, 4); got != 16*time.Second {
		t.Fatalf("retryDelay=%s", got)
	}
	if got := retryDelay(40*time.Minute, 4); got != time.Hour {
		t.Fatalf("capped retryDelay=%s", got)
	}
}

var _ Service = (*testService)(nil)
var _ AlertEnqueuer = (*testAlerts)(nil)
var _ EntitlementEnqueuer = (*testEntitlements)(nil)
