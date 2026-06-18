package alert

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	appalert "github.com/ssubedir/open-spanner/internal/metering/app/alert"
	"github.com/ssubedir/open-spanner/internal/metering/domain"
)

type Service interface {
	EnqueueDueRules(ctx context.Context, limit int) (int, error)
	ClaimEvaluationJob(ctx context.Context, cmd appalert.ClaimCommand) (appalert.EvaluationJobResult, bool, error)
	Evaluate(ctx context.Context, cmd appalert.EvaluateCommand) (appalert.EvaluationResult, error)
	CompleteEvaluationJob(ctx context.Context, cmd appalert.CompleteCommand) error
	FailEvaluationJob(ctx context.Context, cmd appalert.FailCommand) error
	RecordDelivery(ctx context.Context, cmd appalert.DeliveryCommand) (appalert.DeliveryResult, error)
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
	if err != nil || !ok {
		return ok, err
	}

	startedAt := time.Now()
	jobCtx := ctx
	cancel := func() {}
	if w.timeout > 0 {
		jobCtx, cancel = context.WithTimeout(ctx, w.timeout)
	}
	defer cancel()

	result, err := w.service.Evaluate(jobCtx, appalert.EvaluateCommand{RuleID: job.RuleID})
	duration := time.Since(startedAt).Round(time.Millisecond)
	if err == nil {
		for _, event := range resultEvents(result) {
			attempt, attempted := deliverWebhookTrigger(jobCtx, result, event)
			if !attempted {
				continue
			}
			if _, recordErr := w.service.RecordDelivery(ctx, appalert.DeliveryCommand{
				EventID:     event.ID,
				TriggerType: result.Rule.TriggerType,
				Status:      string(attempt.status),
				StatusCode:  attempt.statusCode,
				Error:       attempt.message,
				Duration:    attempt.duration,
				AttemptedAt: attempt.attemptedAt,
			}); recordErr != nil {
				w.logger("alert trigger delivery record failed: rule_id=%s event_id=%s error=%v", job.RuleID, event.ID, recordErr)
			}
			if attempt.status == appalert.DeliveryFailed {
				w.logger("alert trigger delivery failed: rule_id=%s event_id=%s error=%s", job.RuleID, event.ID, attempt.message)
			}
		}
		if err := w.service.CompleteEvaluationJob(ctx, appalert.CompleteCommand{RuleID: job.RuleID}); err != nil && !errors.Is(err, domain.ErrNotFound) {
			return true, err
		}
		w.logger("alert evaluation completed: rule_id=%s status=%s value=%.4f duration=%s", job.RuleID, result.State.Status, result.State.Value, duration)
		return true, nil
	}
	if ctx.Err() != nil && errors.Is(err, context.Canceled) {
		w.logger("alert evaluation abandoned during shutdown: rule_id=%s duration=%s", job.RuleID, duration)
		return true, nil
	}

	failCtx, failCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer failCancel()
	if job.Attempts >= w.maxAttempts {
		if completeErr := w.service.CompleteEvaluationJob(failCtx, appalert.CompleteCommand{RuleID: job.RuleID}); completeErr != nil && !errors.Is(completeErr, domain.ErrNotFound) {
			return true, errors.Join(err, completeErr)
		}
		w.logger("alert evaluation failed permanently: rule_id=%s attempts=%d duration=%s error=%v", job.RuleID, job.Attempts, duration, err)
		return true, nil
	}
	if failErr := w.service.FailEvaluationJob(failCtx, appalert.FailCommand{
		RuleID:     job.RuleID,
		RetryAfter: w.retryAfter,
		Error:      err.Error(),
	}); failErr != nil && !errors.Is(failErr, domain.ErrNotFound) {
		return true, errors.Join(err, failErr)
	}
	w.logger("alert evaluation failed and requeued: rule_id=%s attempts=%d duration=%s error=%v", job.RuleID, job.Attempts, duration, err)
	return true, nil
}

type deliveryAttempt struct {
	status      appalert.DeliveryStatus
	statusCode  int
	message     string
	duration    time.Duration
	attemptedAt time.Time
}

func deliverWebhookTrigger(ctx context.Context, result appalert.EvaluationResult, event appalert.EventResult) (deliveryAttempt, bool) {
	if result.Rule.TriggerType != string(appalert.TriggerWebhook) {
		return deliveryAttempt{}, false
	}

	attemptedAt := time.Now().UTC()
	if result.Rule.WebhookURL == "" {
		return deliveryAttempt{
			status:      appalert.DeliveryFailed,
			message:     "webhook url is not configured",
			attemptedAt: attemptedAt,
		}, true
	}

	state := stateForEvent(result, event)
	payload := webhookPayload{
		Rule: webhookRulePayload{
			ID:                        result.Rule.ID,
			Name:                      result.Rule.Name,
			Meter:                     result.Rule.MeterName,
			Enabled:                   result.Rule.Enabled,
			Subject:                   result.Rule.Subject,
			Metadata:                  result.Rule.Metadata,
			WindowSeconds:             result.Rule.WindowSeconds,
			Comparator:                result.Rule.Comparator,
			Threshold:                 result.Rule.Threshold,
			EvaluationIntervalSeconds: result.Rule.EvaluationInterval,
			GroupBy:                   result.Rule.GroupBy,
		},
		State: webhookStatePayload{
			Status:      state.Status,
			GroupKey:    state.GroupKey,
			GroupValue:  state.GroupValue,
			Value:       state.Value,
			Message:     state.Message,
			EvaluatedAt: state.EvaluatedAt.Format(time.RFC3339),
			UpdatedAt:   state.UpdatedAt.Format(time.RFC3339),
		},
		Event: webhookEventPayload{
			ID:         event.ID,
			RuleID:     event.RuleID,
			GroupKey:   event.GroupKey,
			GroupValue: event.GroupValue,
			Type:       event.Type,
			Value:      event.Value,
			Message:    event.Message,
			CreatedAt:  event.CreatedAt.Format(time.RFC3339),
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return deliveryAttempt{
			status:      appalert.DeliveryFailed,
			message:     err.Error(),
			attemptedAt: attemptedAt,
		}, true
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, result.Rule.WebhookURL, bytes.NewReader(body))
	if err != nil {
		return deliveryAttempt{
			status:      appalert.DeliveryFailed,
			message:     err.Error(),
			attemptedAt: attemptedAt,
		}, true
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "open-spanner-alert-worker")

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
		}, true
	}
	defer res.Body.Close()
	if res.StatusCode < http.StatusOK || res.StatusCode >= http.StatusMultipleChoices {
		return deliveryAttempt{
			status:      appalert.DeliveryFailed,
			statusCode:  res.StatusCode,
			message:     fmt.Sprintf("webhook returned status %d", res.StatusCode),
			duration:    duration,
			attemptedAt: attemptedAt,
		}, true
	}
	return deliveryAttempt{
		status:      appalert.DeliveryDelivered,
		statusCode:  res.StatusCode,
		duration:    duration,
		attemptedAt: attemptedAt,
	}, true
}

func resultEvents(result appalert.EvaluationResult) []appalert.EventResult {
	if len(result.Events) > 0 {
		return result.Events
	}
	if result.Event == nil {
		return nil
	}
	return []appalert.EventResult{*result.Event}
}

func stateForEvent(result appalert.EvaluationResult, event appalert.EventResult) appalert.StateResult {
	for _, state := range result.Rule.States {
		if state.GroupKey == event.GroupKey && state.GroupValue == event.GroupValue {
			return state
		}
	}
	if result.State.GroupKey == event.GroupKey && result.State.GroupValue == event.GroupValue {
		return result.State
	}
	return result.State
}

type webhookPayload struct {
	Event webhookEventPayload `json:"event"`
	Rule  webhookRulePayload  `json:"rule"`
	State webhookStatePayload `json:"state"`
}

type webhookRulePayload struct {
	ID                        string            `json:"id"`
	Name                      string            `json:"name"`
	Meter                     string            `json:"meter"`
	Enabled                   bool              `json:"enabled"`
	Subject                   string            `json:"subject,omitempty"`
	Metadata                  map[string]string `json:"metadata,omitempty"`
	WindowSeconds             int               `json:"window_seconds"`
	Comparator                string            `json:"comparator"`
	Threshold                 float64           `json:"threshold"`
	EvaluationIntervalSeconds int               `json:"evaluation_interval_seconds"`
	GroupBy                   string            `json:"group_by,omitempty"`
}

type webhookStatePayload struct {
	Status      string  `json:"status"`
	GroupKey    string  `json:"group_key,omitempty"`
	GroupValue  string  `json:"group_value,omitempty"`
	Value       float64 `json:"value"`
	Message     string  `json:"message"`
	EvaluatedAt string  `json:"evaluated_at"`
	UpdatedAt   string  `json:"updated_at"`
}

type webhookEventPayload struct {
	ID         string  `json:"id"`
	RuleID     string  `json:"rule_id"`
	GroupKey   string  `json:"group_key,omitempty"`
	GroupValue string  `json:"group_value,omitempty"`
	Type       string  `json:"type"`
	Value      float64 `json:"value"`
	Message    string  `json:"message"`
	CreatedAt  string  `json:"created_at"`
}
