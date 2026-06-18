package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"github.com/ssubedir/open-spanner/internal/metering/adapters/sqlite/sqlitedb"
	appalert "github.com/ssubedir/open-spanner/internal/metering/app/alert"
	"github.com/ssubedir/open-spanner/internal/metering/domain"
)

type AlertRepository struct {
	queries *sqlitedb.Queries
}

func NewAlertRepository(store *Store) *AlertRepository {
	return &AlertRepository{queries: sqlitedb.New(store)}
}

func (r *AlertRepository) SaveRule(ctx context.Context, rule appalert.Rule) (appalert.Rule, error) {
	metadata, err := json.Marshal(rule.Metadata)
	if err != nil {
		return appalert.Rule{}, err
	}

	err = queriesFor(ctx, r.queries).SaveAlertRule(ctx, sqlitedb.SaveAlertRuleParams{
		ID:                        rule.ID,
		Name:                      rule.Name,
		MeterName:                 rule.MeterName,
		Enabled:                   int64(boolInt(rule.Enabled)),
		Subject:                   rule.Subject,
		Metadata:                  string(metadata),
		WindowSeconds:             int64(rule.Window.Seconds()),
		Comparator:                string(rule.Comparator),
		Threshold:                 rule.Threshold,
		EvaluationIntervalSeconds: int64(rule.EvaluationInterval.Seconds()),
		GroupBy:                   rule.GroupBy,
		TriggerType:               string(rule.TriggerType),
		WebhookUrl:                rule.WebhookURL,
		NextEvaluateAt:            formatTime(rule.NextEvaluateAt),
		CreatedAt:                 formatTime(rule.CreatedAt),
		UpdatedAt:                 formatTime(rule.UpdatedAt),
	})
	if err != nil {
		return appalert.Rule{}, err
	}

	return rule, nil
}

func (r *AlertRepository) FindRules(ctx context.Context, query appalert.RuleQuery) ([]appalert.Rule, error) {
	rows, err := queriesFor(ctx, r.queries).ListAlertRules(ctx, sqlitedb.ListAlertRulesParams{
		ID:        alertStringValue(query.ID),
		MeterName: alertStringValue(query.MeterName),
		Enabled:   alertBoolIntValue(query.Enabled),
		Limit:     int64(query.Limit),
	})
	if err != nil {
		return nil, err
	}

	rules := make([]appalert.Rule, 0, len(rows))
	for _, row := range rows {
		rule, err := sqliteAlertRule(row)
		if err != nil {
			return nil, err
		}
		rules = append(rules, rule)
	}
	return rules, nil
}

func (r *AlertRepository) DeleteRule(ctx context.Context, id string) error {
	rows, err := queriesFor(ctx, r.queries).DeleteAlertRule(ctx, id)
	if err != nil {
		return err
	}
	if rows == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *AlertRepository) SaveState(ctx context.Context, state appalert.State) (appalert.State, error) {
	err := queriesFor(ctx, r.queries).SaveAlertState(ctx, sqlitedb.SaveAlertStateParams{
		RuleID:      state.RuleID,
		GroupKey:    state.GroupKey,
		GroupValue:  state.GroupValue,
		Status:      string(state.Status),
		Value:       state.Value,
		Message:     state.Message,
		EvaluatedAt: alertTimeValue(state.EvaluatedAt),
		UpdatedAt:   formatTime(state.UpdatedAt),
	})
	if err != nil {
		return appalert.State{}, err
	}
	return state, nil
}

func (r *AlertRepository) FindState(ctx context.Context, ruleID string, groupKey string, groupValue string) (appalert.State, bool, error) {
	row, err := queriesFor(ctx, r.queries).FindAlertState(ctx, sqlitedb.FindAlertStateParams{
		RuleID:     ruleID,
		GroupKey:   groupKey,
		GroupValue: groupValue,
	})
	if errors.Is(err, sql.ErrNoRows) {
		return appalert.State{}, false, nil
	}
	if err != nil {
		return appalert.State{}, false, err
	}

	state, err := sqliteAlertState(row)
	if err != nil {
		return appalert.State{}, false, err
	}
	return state, true, nil
}

func (r *AlertRepository) FindStates(ctx context.Context, ruleID string, limit int) ([]appalert.State, error) {
	rows, err := queriesFor(ctx, r.queries).ListAlertStates(ctx, sqlitedb.ListAlertStatesParams{
		RuleID: ruleID,
		Limit:  int64(limit),
	})
	if err != nil {
		return nil, err
	}

	states := make([]appalert.State, 0, len(rows))
	for _, row := range rows {
		state, err := sqliteAlertState(row)
		if err != nil {
			return nil, err
		}
		states = append(states, state)
	}
	return states, nil
}

func (r *AlertRepository) SaveEvent(ctx context.Context, event appalert.Event) (appalert.Event, error) {
	err := queriesFor(ctx, r.queries).SaveAlertEvent(ctx, sqlitedb.SaveAlertEventParams{
		ID:         event.ID,
		RuleID:     event.RuleID,
		GroupKey:   event.GroupKey,
		GroupValue: event.GroupValue,
		Type:       string(event.Type),
		Value:      event.Value,
		Message:    event.Message,
		CreatedAt:  formatTime(event.CreatedAt),
	})
	if err != nil {
		return appalert.Event{}, err
	}
	return event, nil
}

func (r *AlertRepository) SaveDelivery(ctx context.Context, delivery appalert.Delivery) (appalert.Delivery, error) {
	err := queriesFor(ctx, r.queries).SaveAlertDelivery(ctx, sqlitedb.SaveAlertDeliveryParams{
		ID:          delivery.ID,
		EventID:     delivery.EventID,
		TriggerType: string(delivery.TriggerType),
		Status:      string(delivery.Status),
		StatusCode:  alertIntValue(delivery.StatusCode),
		Error:       delivery.Error,
		DurationMs:  delivery.Duration.Milliseconds(),
		AttemptedAt: formatTime(delivery.AttemptedAt),
		CreatedAt:   formatTime(delivery.CreatedAt),
	})
	if err != nil {
		return appalert.Delivery{}, err
	}
	return delivery, nil
}

func (r *AlertRepository) FindEvents(ctx context.Context, query appalert.EventQuery) ([]appalert.Event, error) {
	rows, err := queriesFor(ctx, r.queries).ListAlertEvents(ctx, sqlitedb.ListAlertEventsParams{
		RuleID:          alertStringValue(query.RuleID),
		CursorCreatedAt: alertTimeValue(query.CreatedAt),
		CursorID:        alertStringValue(query.ID),
		Limit:           int64(query.Limit),
	})
	if err != nil {
		return nil, err
	}

	events := make([]appalert.Event, 0, len(rows))
	for _, row := range rows {
		event, err := sqliteAlertEvent(row)
		if err != nil {
			return nil, err
		}
		events = append(events, event)
	}
	return events, nil
}

func (r *AlertRepository) EnqueueEvaluationJob(ctx context.Context, ruleID string, runAfter time.Time, now time.Time) error {
	return queriesFor(ctx, r.queries).EnqueueAlertEvaluationJob(ctx, sqlitedb.EnqueueAlertEvaluationJobParams{
		RuleID:   ruleID,
		RunAfter: formatTime(runAfter),
		Now:      formatTime(now),
	})
}

func (r *AlertRepository) EnqueueDueEvaluationJobs(ctx context.Context, now time.Time, limit int) (int, error) {
	rows, err := queriesFor(ctx, r.queries).EnqueueDueAlertEvaluationJobs(ctx, sqlitedb.EnqueueDueAlertEvaluationJobsParams{
		RunAfter: formatTime(now),
		Now:      formatTime(now),
		Limit:    int64(limit),
	})
	return int(rows), err
}

func (r *AlertRepository) ClaimEvaluationJob(ctx context.Context, now time.Time, lockedUntil time.Time, maxAttempts int) (appalert.EvaluationJob, error) {
	row, err := queriesFor(ctx, r.queries).ClaimAlertEvaluationJob(ctx, sqlitedb.ClaimAlertEvaluationJobParams{
		LockedUntil: alertTimeValue(lockedUntil),
		Now:         formatTime(now),
		MaxAttempts: int64(maxAttempts),
	})
	if errors.Is(err, sql.ErrNoRows) {
		return appalert.EvaluationJob{}, domain.ErrNotFound
	}
	if err != nil {
		return appalert.EvaluationJob{}, err
	}
	return sqliteAlertEvaluationJob(row)
}

func (r *AlertRepository) CompleteEvaluationJob(ctx context.Context, ruleID string) error {
	rows, err := queriesFor(ctx, r.queries).DeleteAlertEvaluationJob(ctx, ruleID)
	if err != nil {
		return err
	}
	if rows == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *AlertRepository) RequeueEvaluationJob(ctx context.Context, ruleID string, runAfter time.Time, now time.Time) error {
	rows, err := queriesFor(ctx, r.queries).RequeueAlertEvaluationJob(ctx, sqlitedb.RequeueAlertEvaluationJobParams{
		RuleID:   ruleID,
		RunAfter: formatTime(runAfter),
		Now:      formatTime(now),
	})
	if err != nil {
		return err
	}
	if rows == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *AlertRepository) UpdateRuleNextEvaluation(ctx context.Context, id string, nextEvaluateAt time.Time, updatedAt time.Time) error {
	rows, err := queriesFor(ctx, r.queries).UpdateAlertRuleNextEvaluation(ctx, sqlitedb.UpdateAlertRuleNextEvaluationParams{
		ID:             id,
		NextEvaluateAt: formatTime(nextEvaluateAt),
		UpdatedAt:      formatTime(updatedAt),
	})
	if err != nil {
		return err
	}
	if rows == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func sqliteAlertRule(row sqlitedb.ListAlertRulesRow) (appalert.Rule, error) {
	metadata := map[string]string{}
	if row.Metadata != "" {
		if err := json.Unmarshal([]byte(row.Metadata), &metadata); err != nil {
			return appalert.Rule{}, err
		}
	}
	nextEvaluateAt, err := time.Parse(time.RFC3339Nano, row.NextEvaluateAt)
	if err != nil {
		return appalert.Rule{}, err
	}
	createdAt, err := time.Parse(time.RFC3339Nano, row.CreatedAt)
	if err != nil {
		return appalert.Rule{}, err
	}
	updatedAt, err := time.Parse(time.RFC3339Nano, row.UpdatedAt)
	if err != nil {
		return appalert.Rule{}, err
	}

	return appalert.Rule{
		ID:                 row.ID,
		Name:               row.Name,
		MeterName:          row.MeterName,
		Enabled:            row.Enabled != 0,
		Subject:            row.Subject,
		Metadata:           metadata,
		Window:             time.Duration(row.WindowSeconds) * time.Second,
		Comparator:         appalert.Comparator(row.Comparator),
		Threshold:          row.Threshold,
		EvaluationInterval: time.Duration(row.EvaluationIntervalSeconds) * time.Second,
		GroupBy:            row.GroupBy,
		TriggerType:        appalert.TriggerType(row.TriggerType),
		WebhookURL:         row.WebhookUrl,
		NextEvaluateAt:     nextEvaluateAt,
		CreatedAt:          createdAt,
		UpdatedAt:          updatedAt,
	}, nil
}

func sqliteAlertState(row sqlitedb.AlertState) (appalert.State, error) {
	evaluatedAt := time.Time{}
	var err error
	if row.EvaluatedAt.Valid {
		evaluatedAt, err = time.Parse(time.RFC3339Nano, row.EvaluatedAt.String)
		if err != nil {
			return appalert.State{}, err
		}
	}
	updatedAt, err := time.Parse(time.RFC3339Nano, row.UpdatedAt)
	if err != nil {
		return appalert.State{}, err
	}

	return appalert.State{
		RuleID:      row.RuleID,
		GroupKey:    row.GroupKey,
		GroupValue:  row.GroupValue,
		Status:      appalert.StateStatus(row.Status),
		Value:       row.Value,
		Message:     row.Message,
		EvaluatedAt: evaluatedAt,
		UpdatedAt:   updatedAt,
	}, nil
}

func sqliteAlertEvent(row sqlitedb.ListAlertEventsRow) (appalert.Event, error) {
	createdAt, err := time.Parse(time.RFC3339Nano, row.CreatedAt)
	if err != nil {
		return appalert.Event{}, err
	}
	return appalert.Event{
		ID:         row.ID,
		RuleID:     row.RuleID,
		GroupKey:   row.GroupKey,
		GroupValue: row.GroupValue,
		Type:       appalert.EventType(row.Type),
		Value:      row.Value,
		Message:    row.Message,
		CreatedAt:  createdAt,
		Delivery:   sqliteAlertDelivery(row),
	}, nil
}

func sqliteAlertDelivery(row sqlitedb.ListAlertEventsRow) *appalert.Delivery {
	if !row.DeliveryID.Valid {
		return nil
	}

	attemptedAt := time.Time{}
	if row.DeliveryAttemptedAt.Valid {
		parsed, err := time.Parse(time.RFC3339Nano, row.DeliveryAttemptedAt.String)
		if err == nil {
			attemptedAt = parsed
		}
	}
	createdAt := time.Time{}
	if row.DeliveryCreatedAt.Valid {
		parsed, err := time.Parse(time.RFC3339Nano, row.DeliveryCreatedAt.String)
		if err == nil {
			createdAt = parsed
		}
	}
	statusCode := 0
	if row.DeliveryStatusCode.Valid {
		statusCode = int(row.DeliveryStatusCode.Int64)
	}
	duration := time.Duration(0)
	if row.DeliveryDurationMs.Valid {
		duration = time.Duration(row.DeliveryDurationMs.Int64) * time.Millisecond
	}

	return &appalert.Delivery{
		ID:          row.DeliveryID.String,
		EventID:     row.ID,
		TriggerType: appalert.TriggerType(row.DeliveryTriggerType.String),
		Status:      appalert.DeliveryStatus(row.DeliveryStatus.String),
		StatusCode:  statusCode,
		Error:       row.DeliveryError.String,
		Duration:    duration,
		AttemptedAt: attemptedAt,
		CreatedAt:   createdAt,
	}
}

func sqliteAlertEvaluationJob(row sqlitedb.AlertEvaluationJob) (appalert.EvaluationJob, error) {
	runAfter, err := time.Parse(time.RFC3339Nano, row.RunAfter)
	if err != nil {
		return appalert.EvaluationJob{}, err
	}
	lockedUntil := time.Time{}
	if row.LockedUntil.Valid {
		lockedUntil, err = time.Parse(time.RFC3339Nano, row.LockedUntil.String)
		if err != nil {
			return appalert.EvaluationJob{}, err
		}
	}
	createdAt, err := time.Parse(time.RFC3339Nano, row.CreatedAt)
	if err != nil {
		return appalert.EvaluationJob{}, err
	}
	updatedAt, err := time.Parse(time.RFC3339Nano, row.UpdatedAt)
	if err != nil {
		return appalert.EvaluationJob{}, err
	}

	return appalert.EvaluationJob{
		RuleID:      row.RuleID,
		RunAfter:    runAfter,
		LockedUntil: lockedUntil,
		Attempts:    int(row.Attempts),
		CreatedAt:   createdAt,
		UpdatedAt:   updatedAt,
	}, nil
}

func alertStringValue(value string) sql.NullString {
	if value == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: value, Valid: true}
}

func alertTimeValue(value time.Time) sql.NullString {
	if value.IsZero() {
		return sql.NullString{}
	}
	return sql.NullString{String: formatTime(value), Valid: true}
}

func alertBoolIntValue(value *bool) sql.NullInt64 {
	if value == nil {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: int64(boolInt(*value)), Valid: true}
}

func alertIntValue(value int) sql.NullInt64 {
	if value == 0 {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: int64(value), Valid: true}
}
