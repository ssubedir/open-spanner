package alert

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"sync"
	"time"

	appauth "github.com/ssubedir/open-spanner/internal/auth"
	appalert "github.com/ssubedir/open-spanner/internal/metering/app/alert"
	"github.com/ssubedir/open-spanner/internal/metering/domain"
)

type Service interface {
	EnqueueDueRules(ctx context.Context, limit int) (int, error)
	ClaimEvaluationJob(ctx context.Context, cmd appalert.ClaimCommand) (appalert.EvaluationJobResult, bool, error)
	Evaluate(ctx context.Context, cmd appalert.EvaluateCommand) (appalert.EvaluationResult, error)
	CompleteEvaluationJob(ctx context.Context, cmd appalert.CompleteCommand) error
	FailEvaluationJob(ctx context.Context, cmd appalert.FailCommand) error
	DeadLetterEvaluationJob(ctx context.Context, cmd appalert.DeadLetterCommand) error
	ClaimDeliveryJob(ctx context.Context, cmd appalert.ClaimCommand) (appalert.DeliveryJobResult, bool, error)
	CompleteDeliveryJob(ctx context.Context, cmd appalert.DeliveryJobCompleteCommand) error
	FailDeliveryJob(ctx context.Context, cmd appalert.DeliveryJobFailCommand) error
}

type Logger func(format string, args ...any)

type Worker struct {
	service     Service
	interval    time.Duration
	lockTTL     time.Duration
	timeout     time.Duration
	retryAfter  time.Duration
	maxAttempts int
	batchSize   int
	logger      Logger
}

func NewWorker(service Service, interval time.Duration, lockTTL time.Duration, timeout time.Duration, retryAfter time.Duration, maxAttempts int, batchSize int, logger Logger) *Worker {
	if logger == nil {
		logger = log.Printf
	}
	return &Worker{
		service:     service,
		interval:    interval,
		lockTTL:     lockTTL,
		timeout:     timeout,
		retryAfter:  retryAfter,
		maxAttempts: maxAttempts,
		batchSize:   batchSize,
		logger:      logger,
	}
}

func (w *Worker) Start(ctx context.Context) func() {
	workerCtx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})

	go func() {
		defer close(done)
		w.run(workerCtx)
	}()

	var once sync.Once
	return func() {
		once.Do(func() {
			cancel()
			<-done
		})
	}
}

func (w *Worker) run(ctx context.Context) {
	if w.service == nil || w.interval <= 0 {
		return
	}

	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	w.logger("alert worker started: interval=%s lock_ttl=%s timeout=%s retry_after=%s max_attempts=%d batch_size=%d", w.interval, w.lockTTL, w.timeout, w.retryAfter, w.maxAttempts, w.batchSize)
	defer w.logger("alert worker stopped")

	for {
		w.drain(ctx)

		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (w *Worker) drain(ctx context.Context) {
	if _, err := w.service.EnqueueDueRules(ctx, w.batchSize); err != nil {
		w.logger("alert due rule enqueue failed: error=%v", err)
		return
	}

	for processed := 0; processed < w.batchSize; processed++ {
		ok, err := w.ProcessOnce(ctx)
		if err != nil {
			w.logger("alert evaluation processing failed: error=%v", err)
			return
		}
		if !ok {
			return
		}
	}
}

func (w *Worker) ProcessOnce(ctx context.Context) (bool, error) {
	job, ok, err := w.service.ClaimEvaluationJob(ctx, appalert.ClaimCommand{
		LockTTL:     w.lockTTL,
		MaxAttempts: w.maxAttempts,
	})
	if err != nil {
		return false, err
	}
	if !ok {
		return w.processDeliveryOnce(ctx)
	}

	startedAt := time.Now()
	baseCtx := appauth.WithWorkspaceID(ctx, job.WorkspaceID)
	jobCtx := baseCtx
	cancel := func() {}
	if w.timeout > 0 {
		jobCtx, cancel = context.WithTimeout(baseCtx, w.timeout)
	}
	defer cancel()

	result, err := w.service.Evaluate(jobCtx, appalert.EvaluateCommand{RuleID: job.RuleID})
	duration := time.Since(startedAt).Round(time.Millisecond)
	if err == nil {
		if err := w.service.CompleteEvaluationJob(baseCtx, appalert.CompleteCommand{RuleID: job.RuleID, Attempts: job.Attempts}); err != nil && !errors.Is(err, domain.ErrNotFound) {
			return true, err
		}
		w.logger("alert evaluation completed: rule_id=%s status=%s value=%.4f duration=%s", job.RuleID, result.State.Status, result.State.Value, duration)
		return true, nil
	}
	if ctx.Err() != nil && errors.Is(err, context.Canceled) {
		w.logger("alert evaluation abandoned during shutdown: rule_id=%s duration=%s", job.RuleID, duration)
		return true, nil
	}

	failCtx, failCancel := context.WithTimeout(appauth.WithWorkspaceID(context.Background(), job.WorkspaceID), 10*time.Second)
	defer failCancel()
	if job.Attempts >= w.maxAttempts {
		if deadLetterErr := w.service.DeadLetterEvaluationJob(failCtx, appalert.DeadLetterCommand{RuleID: job.RuleID, Attempts: job.Attempts, Error: err.Error()}); deadLetterErr != nil && !errors.Is(deadLetterErr, domain.ErrNotFound) {
			return true, errors.Join(err, deadLetterErr)
		}
		w.logger("alert evaluation failed permanently: rule_id=%s attempts=%d duration=%s error=%v", job.RuleID, job.Attempts, duration, err)
		return true, nil
	}
	if failErr := w.service.FailEvaluationJob(failCtx, appalert.FailCommand{
		RuleID:     job.RuleID,
		Attempts:   job.Attempts,
		RetryAfter: w.retryAfter,
		Error:      err.Error(),
	}); failErr != nil && !errors.Is(failErr, domain.ErrNotFound) {
		return true, errors.Join(err, failErr)
	}
	w.logger("alert evaluation failed and requeued: rule_id=%s attempts=%d duration=%s error=%v", job.RuleID, job.Attempts, duration, err)
	return true, nil
}

func (w *Worker) processDeliveryOnce(ctx context.Context) (bool, error) {
	job, ok, err := w.service.ClaimDeliveryJob(ctx, appalert.ClaimCommand{LockTTL: w.lockTTL, MaxAttempts: w.maxAttempts})
	if err != nil || !ok {
		return ok, err
	}
	baseCtx := appauth.WithWorkspaceID(ctx, job.WorkspaceID)
	jobCtx := baseCtx
	cancel := func() {}
	if w.timeout > 0 {
		jobCtx, cancel = context.WithTimeout(baseCtx, w.timeout)
	}
	defer cancel()
	attempt := deliverWebhookJob(jobCtx, job)
	delivery := appalert.DeliveryCommand{EventID: job.EventID, TriggerType: string(appalert.TriggerWebhook), Status: string(attempt.status), StatusCode: attempt.statusCode, Error: attempt.message, Duration: attempt.duration, AttemptedAt: attempt.attemptedAt}
	if attempt.status == appalert.DeliveryDelivered {
		if err := w.service.CompleteDeliveryJob(baseCtx, appalert.DeliveryJobCompleteCommand{ID: job.ID, Attempts: job.Attempts, Delivery: delivery}); err != nil && !errors.Is(err, domain.ErrNotFound) {
			return true, err
		}
		w.logger("alert delivery completed: delivery_id=%s event_id=%s attempts=%d", job.ID, job.EventID, job.Attempts)
		return true, nil
	}
	if ctx.Err() != nil && errors.Is(ctx.Err(), context.Canceled) {
		return true, nil
	}
	failCtx, failCancel := context.WithTimeout(appauth.WithWorkspaceID(context.Background(), job.WorkspaceID), 10*time.Second)
	defer failCancel()
	err = w.service.FailDeliveryJob(failCtx, appalert.DeliveryJobFailCommand{ID: job.ID, Attempts: job.Attempts, MaxAttempts: w.maxAttempts, RetryAfter: w.retryAfter, Delivery: delivery})
	if err != nil && !errors.Is(err, domain.ErrNotFound) {
		return true, err
	}
	if job.Attempts >= w.maxAttempts {
		w.logger("alert delivery failed permanently: delivery_id=%s event_id=%s attempts=%d error=%s", job.ID, job.EventID, job.Attempts, attempt.message)
	} else {
		w.logger("alert delivery failed and requeued: delivery_id=%s event_id=%s attempts=%d error=%s", job.ID, job.EventID, job.Attempts, attempt.message)
	}
	return true, nil
}

type deliveryAttempt struct {
	status      appalert.DeliveryStatus
	statusCode  int
	message     string
	duration    time.Duration
	attemptedAt time.Time
}

func deliverWebhookJob(ctx context.Context, job appalert.DeliveryJobResult) deliveryAttempt {
	attemptedAt := time.Now().UTC()
	target, targetErr := webhookDeliveryTarget(job.Destination)
	if targetErr != "" {
		return deliveryAttempt{
			status:      appalert.DeliveryFailed,
			message:     targetErr,
			attemptedAt: attemptedAt,
		}
	}
	body := job.Payload
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, target.url, bytes.NewReader(body))
	if err != nil {
		return deliveryAttempt{
			status:      appalert.DeliveryFailed,
			message:     err.Error(),
			attemptedAt: attemptedAt,
		}
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "open-spanner-alert-worker")
	req.Header.Set("X-Open-Spanner-Delivery-ID", job.ID)
	if target.secret != "" {
		signWebhookRequest(req, target.secret, attemptedAt, body)
	}

	client := http.Client{Timeout: 10 * time.Second}
	startedAt := time.Now()
	res, err := client.Do(req)
	duration := time.Since(startedAt).Round(time.Millisecond)
	if err != nil {
		return deliveryAttempt{
			status:      appalert.DeliveryFailed,
			message:     err.Error(),
			duration:    duration,
			attemptedAt: attemptedAt,
		}
	}
	defer res.Body.Close()
	if res.StatusCode < http.StatusOK || res.StatusCode >= http.StatusMultipleChoices {
		return deliveryAttempt{
			status:      appalert.DeliveryFailed,
			statusCode:  res.StatusCode,
			message:     fmt.Sprintf("webhook returned status %d", res.StatusCode),
			duration:    duration,
			attemptedAt: attemptedAt,
		}
	}
	return deliveryAttempt{
		status:      appalert.DeliveryDelivered,
		statusCode:  res.StatusCode,
		duration:    duration,
		attemptedAt: attemptedAt,
	}
}

type webhookDeliveryTargetValue struct {
	url    string
	secret string
}

func webhookDeliveryTarget(destination *appalert.DestinationResult) (webhookDeliveryTargetValue, string) {
	if destination == nil {
		return webhookDeliveryTargetValue{}, "alert destination is not configured"
	}
	if !destination.Enabled {
		return webhookDeliveryTargetValue{}, "alert destination is disabled"
	}
	if destination.WebhookURL == "" {
		return webhookDeliveryTargetValue{}, "alert destination webhook url is not configured"
	}
	return webhookDeliveryTargetValue{url: destination.WebhookURL, secret: destination.WebhookSecret}, ""
}

func signWebhookRequest(req *http.Request, secret string, timestamp time.Time, body []byte) {
	timestampValue := strconv.FormatInt(timestamp.Unix(), 10)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(timestampValue))
	mac.Write([]byte("."))
	mac.Write(body)
	req.Header.Set(appalert.WebhookTimestampHeader, timestampValue)
	req.Header.Set(appalert.WebhookSignatureHeader, appalert.WebhookSignatureVersion+"="+hex.EncodeToString(mac.Sum(nil)))
}
