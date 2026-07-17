package alert

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	appauth "github.com/ssubedir/open-spanner/internal/auth"
	"github.com/ssubedir/open-spanner/internal/metering/domain"
)

func (s *service) ClaimDeliveryJob(ctx context.Context, cmd ClaimCommand) (DeliveryJobResult, bool, error) {
	if cmd.LockTTL <= 0 {
		cmd.LockTTL = time.Minute
	}
	if cmd.MaxAttempts <= 0 {
		cmd.MaxAttempts = 5
	}
	now := s.now()
	job, err := s.repo.ClaimDeliveryJob(ctx, now, now.Add(cmd.LockTTL), cmd.MaxAttempts)
	if errors.Is(err, domain.ErrNotFound) {
		return DeliveryJobResult{}, false, nil
	}
	if err != nil {
		return DeliveryJobResult{}, false, err
	}
	result := deliveryJobResult(job)
	if job.DestinationID != "" {
		destination, err := s.findDestination(appauth.WithWorkspaceID(ctx, job.WorkspaceID), job.DestinationID)
		if err != nil && !errors.Is(err, domain.ErrNotFound) {
			return DeliveryJobResult{}, false, err
		}
		if err == nil {
			value := destinationResult(destination)
			result.Destination = &value
		}
	}
	return result, true, nil
}

func (s *service) CompleteDeliveryJob(ctx context.Context, cmd DeliveryJobCompleteCommand) error {
	id, err := normalizeRequired(cmd.ID, "alert delivery job id is required")
	if err != nil {
		return err
	}
	if cmd.Attempts < 1 {
		return domain.ErrInvalidInput
	}
	return s.transactor.WithinTransaction(ctx, func(txCtx context.Context) error {
		if _, err := s.RecordDelivery(txCtx, cmd.Delivery); err != nil {
			return err
		}
		return s.repo.CompleteDeliveryJob(txCtx, id, cmd.Attempts, s.now())
	})
}

func (s *service) FailDeliveryJob(ctx context.Context, cmd DeliveryJobFailCommand) error {
	id, err := normalizeRequired(cmd.ID, "alert delivery job id is required")
	if err != nil {
		return err
	}
	if cmd.Attempts < 1 || cmd.MaxAttempts < 1 {
		return domain.ErrInvalidInput
	}
	if cmd.RetryAfter <= 0 {
		cmd.RetryAfter = time.Minute
	}
	now := s.now()
	return s.transactor.WithinTransaction(ctx, func(txCtx context.Context) error {
		if _, err := s.RecordDelivery(txCtx, cmd.Delivery); err != nil {
			return err
		}
		return s.repo.RetryDeliveryJob(txCtx, id, cmd.Attempts, now.Add(deliveryRetryDelay(cmd.RetryAfter, cmd.Attempts)), cmd.MaxAttempts, cmd.Delivery.Error, now)
	})
}

func (s *service) ListDeliveryJobs(ctx context.Context, limit int) (DeliveryJobListResult, error) {
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}
	jobs, err := s.repo.ListDeliveryJobs(ctx, limit)
	if err != nil {
		return DeliveryJobListResult{}, err
	}
	result := DeliveryJobListResult{Items: make([]DeliveryJobResult, 0, len(jobs))}
	for _, job := range jobs {
		result.Items = append(result.Items, deliveryJobResult(job))
	}
	return result, nil
}

func (s *service) RequeueDeliveryJob(ctx context.Context, id string) error {
	id, err := normalizeRequired(id, "alert delivery job id is required")
	if err != nil {
		return err
	}
	err = s.repo.RequeueDeliveryJob(ctx, id, s.now())
	if !errors.Is(err, domain.ErrNotFound) {
		return err
	}
	status, statusErr := s.repo.FindDeliveryJobStatus(ctx, id)
	if statusErr != nil {
		return statusErr
	}
	return errors.Join(domain.ErrConflict, fmt.Errorf("alert delivery job is %s", status))
}

func deliveryRetryDelay(base time.Duration, attempts int) time.Duration {
	delay := base
	for i := 1; i < attempts; i++ {
		if delay >= time.Hour/2 {
			return time.Hour
		}
		delay *= 2
	}
	if delay > time.Hour {
		return time.Hour
	}
	return delay
}

func deliveryJobResult(job DeliveryJob) DeliveryJobResult {
	return DeliveryJobResult{ID: job.ID, WorkspaceID: job.WorkspaceID, EventID: job.EventID, DestinationID: job.DestinationID, Payload: append([]byte(nil), job.Payload...), Status: job.Status, Attempts: job.Attempts, NextAttemptAt: job.NextAttemptAt, LastError: job.LastError, CreatedAt: job.CreatedAt, UpdatedAt: job.UpdatedAt, DeliveredAt: job.DeliveredAt}
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
	DestinationID             string            `json:"destination_id,omitempty"`
	DestinationName           string            `json:"destination_name,omitempty"`
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

func deliveryPayload(rule Rule, destination *Destination, state State, event Event) ([]byte, error) {
	destinationName := ""
	if destination != nil {
		destinationName = destination.Name
	}
	return json.Marshal(webhookPayload{
		Rule:  webhookRulePayload{ID: rule.ID, Name: rule.Name, Meter: rule.MeterName, Enabled: rule.Enabled, Subject: rule.Subject, Metadata: rule.Metadata, WindowSeconds: int(rule.Window.Seconds()), Comparator: string(rule.Comparator), Threshold: rule.Threshold, EvaluationIntervalSeconds: int(rule.EvaluationInterval.Seconds()), GroupBy: rule.GroupBy, DestinationID: rule.DestinationID, DestinationName: destinationName},
		State: webhookStatePayload{Status: string(state.Status), GroupKey: state.GroupKey, GroupValue: state.GroupValue, Value: state.Value, Message: state.Message, EvaluatedAt: state.EvaluatedAt.Format(time.RFC3339), UpdatedAt: state.UpdatedAt.Format(time.RFC3339)},
		Event: webhookEventPayload{ID: event.ID, RuleID: event.RuleID, GroupKey: event.GroupKey, GroupValue: event.GroupValue, Type: string(event.Type), Value: event.Value, Message: event.Message, CreatedAt: event.CreatedAt.Format(time.RFC3339)},
	})
}

func newDeliveryJob(event Event, rule Rule, state State, destination *Destination, now time.Time) (DeliveryJob, error) {
	payload, err := deliveryPayload(rule, destination, state, event)
	if err != nil {
		return DeliveryJob{}, err
	}
	return DeliveryJob{ID: uuid.NewString(), EventID: event.ID, DestinationID: rule.DestinationID, Payload: payload, Status: "pending", NextAttemptAt: now, CreatedAt: now, UpdatedAt: now}, nil
}
