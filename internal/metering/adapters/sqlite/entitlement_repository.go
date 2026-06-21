package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"time"

	appauth "github.com/ssubedir/open-spanner/internal/auth"
	"github.com/ssubedir/open-spanner/internal/metering/adapters/sqlite/sqlitedb"
	appentitlement "github.com/ssubedir/open-spanner/internal/metering/app/entitlement"
	"github.com/ssubedir/open-spanner/internal/metering/domain"
)

type EntitlementRepository struct {
	queries *sqlitedb.Queries
}

func NewEntitlementRepository(store *Store) *EntitlementRepository {
	return &EntitlementRepository{queries: sqlitedb.New(store)}
}

func (r *EntitlementRepository) SavePlan(ctx context.Context, plan appentitlement.Plan) (appentitlement.Plan, error) {
	workspaceID, err := appauth.RequireWorkspaceID(ctx)
	if err != nil {
		return appentitlement.Plan{}, err
	}
	err = queriesFor(ctx, r.queries).SavePlan(ctx, sqlitedb.SavePlanParams{
		ID:           plan.ID,
		WorkspaceID:  workspaceID,
		Name:         plan.Name,
		Description:  plan.Description,
		Version:      int64(plan.Version),
		ParentPlanID: planStringValue(plan.ParentPlanID),
		IsCurrent:    sqliteBool(plan.IsCurrent),
		CreatedAt:    formatTime(plan.CreatedAt),
		UpdatedAt:    formatTime(plan.UpdatedAt),
	})
	if err != nil {
		if isUniqueConstraint(err) {
			return appentitlement.Plan{}, errors.Join(domain.ErrConflict, err)
		}
		return appentitlement.Plan{}, err
	}
	return plan, nil
}

func (r *EntitlementRepository) RetirePlan(ctx context.Context, id string, updatedAt time.Time) error {
	workspaceID, err := appauth.RequireWorkspaceID(ctx)
	if err != nil {
		return err
	}
	rows, err := queriesFor(ctx, r.queries).RetirePlan(ctx, sqlitedb.RetirePlanParams{
		UpdatedAt:   formatTime(updatedAt),
		WorkspaceID: workspaceID,
		ID:          id,
	})
	if err != nil {
		return err
	}
	if rows == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *EntitlementRepository) FindPlans(ctx context.Context, query appentitlement.PlanQuery) ([]appentitlement.Plan, error) {
	workspaceID, err := appauth.RequireWorkspaceID(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := queriesFor(ctx, r.queries).ListPlans(ctx, sqlitedb.ListPlansParams{
		WorkspaceID: workspaceID,
		ID:          planStringValue(query.ID),
		Name:        planStringValue(query.Name),
		CurrentOnly: sqliteBool(query.CurrentOnly),
		Limit:       int64(query.Limit),
	})
	if err != nil {
		return nil, err
	}
	plans := make([]appentitlement.Plan, 0, len(rows))
	for _, row := range rows {
		plan, err := sqlitePlan(row)
		if err != nil {
			return nil, err
		}
		plans = append(plans, plan)
	}
	return plans, nil
}

func (r *EntitlementRepository) DeletePlan(ctx context.Context, id string) error {
	workspaceID, err := appauth.RequireWorkspaceID(ctx)
	if err != nil {
		return err
	}
	rows, err := queriesFor(ctx, r.queries).DeletePlan(ctx, sqlitedb.DeletePlanParams{
		WorkspaceID: workspaceID,
		ID:          id,
	})
	if err != nil {
		return err
	}
	if rows == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *EntitlementRepository) ReplacePlanLimits(ctx context.Context, planID string, limits []appentitlement.PlanLimit) error {
	workspaceID, err := appauth.RequireWorkspaceID(ctx)
	if err != nil {
		return err
	}
	if err := queriesFor(ctx, r.queries).DeletePlanLimits(ctx, sqlitedb.DeletePlanLimitsParams{
		WorkspaceID: workspaceID,
		PlanID:      planID,
	}); err != nil {
		return err
	}
	for _, limit := range limits {
		if err := queriesFor(ctx, r.queries).SavePlanLimit(ctx, sqlitedb.SavePlanLimitParams{
			ID:             limit.ID,
			WorkspaceID:    workspaceID,
			PlanID:         planID,
			MeterName:      limit.MeterName,
			Period:         string(limit.Period),
			LimitValue:     limit.Limit,
			WarningPercent: limit.WarningPercent,
			CreatedAt:      formatTime(limit.CreatedAt),
			UpdatedAt:      formatTime(limit.UpdatedAt),
		}); err != nil {
			if isUniqueConstraint(err) {
				return errors.Join(domain.ErrConflict, err)
			}
			return err
		}
	}
	return nil
}

func (r *EntitlementRepository) FindPlanLimits(ctx context.Context, planID string) ([]appentitlement.PlanLimit, error) {
	workspaceID, err := appauth.RequireWorkspaceID(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := queriesFor(ctx, r.queries).ListPlanLimits(ctx, sqlitedb.ListPlanLimitsParams{
		WorkspaceID: workspaceID,
		PlanID:      planStringValue(planID),
	})
	if err != nil {
		return nil, err
	}
	limits := make([]appentitlement.PlanLimit, 0, len(rows))
	for _, row := range rows {
		limit, err := sqlitePlanLimit(row)
		if err != nil {
			return nil, err
		}
		limits = append(limits, limit)
	}
	return limits, nil
}

func (r *EntitlementRepository) CountAssignmentsForPlan(ctx context.Context, planID string) (int, error) {
	workspaceID, err := appauth.RequireWorkspaceID(ctx)
	if err != nil {
		return 0, err
	}
	count, err := queriesFor(ctx, r.queries).CountPlanAssignments(ctx, sqlitedb.CountPlanAssignmentsParams{
		WorkspaceID: workspaceID,
		PlanID:      planID,
	})
	if err != nil {
		return 0, err
	}
	return int(count), nil
}

func (r *EntitlementRepository) SaveSubjectAssignment(ctx context.Context, assignment appentitlement.SubjectAssignment) (appentitlement.SubjectAssignment, error) {
	workspaceID, err := appauth.RequireWorkspaceID(ctx)
	if err != nil {
		return appentitlement.SubjectAssignment{}, err
	}
	now := assignment.UpdatedAt
	if now.IsZero() {
		now = time.Now().UTC()
	}
	endAt := formatTime(now)
	if _, err := queriesFor(ctx, r.queries).EndCurrentPlanSubjectAssignment(ctx, sqlitedb.EndCurrentPlanSubjectAssignmentParams{
		UnassignedAt: sql.NullString{String: endAt, Valid: true},
		UpdatedAt:    endAt,
		WorkspaceID:  workspaceID,
		Subject:      assignment.Subject,
	}); err != nil {
		return appentitlement.SubjectAssignment{}, err
	}
	err = queriesFor(ctx, r.queries).SavePlanSubjectAssignment(ctx, sqlitedb.SavePlanSubjectAssignmentParams{
		ID:             assignment.ID,
		WorkspaceID:    workspaceID,
		Subject:        assignment.Subject,
		PlanID:         assignment.PlanID,
		AssignedAt:     formatTime(assignment.AssignedAt),
		PeriodAnchorAt: formatTime(assignment.PeriodAnchorAt),
		UpdatedAt:      formatTime(assignment.UpdatedAt),
	})
	if err != nil {
		if isUniqueConstraint(err) {
			return appentitlement.SubjectAssignment{}, errors.Join(domain.ErrConflict, err)
		}
		return appentitlement.SubjectAssignment{}, err
	}
	assignments, err := r.FindSubjectAssignments(ctx, appentitlement.AssignmentQuery{Subject: assignment.Subject, PlanID: assignment.PlanID, ActiveOnly: true, Limit: 1})
	if err != nil {
		return appentitlement.SubjectAssignment{}, err
	}
	if len(assignments) == 0 {
		return appentitlement.SubjectAssignment{}, domain.ErrNotFound
	}
	return assignments[0], nil
}

func (r *EntitlementRepository) FindSubjectAssignments(ctx context.Context, query appentitlement.AssignmentQuery) ([]appentitlement.SubjectAssignment, error) {
	workspaceID, err := appauth.RequireWorkspaceID(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := queriesFor(ctx, r.queries).ListPlanSubjectAssignments(ctx, sqlitedb.ListPlanSubjectAssignmentsParams{
		WorkspaceID: workspaceID,
		Subject:     planStringValue(query.Subject),
		PlanID:      planStringValue(query.PlanID),
		ActiveOnly:  sqliteBool(query.ActiveOnly),
		Limit:       int64(query.Limit),
	})
	if err != nil {
		return nil, err
	}
	assignments := make([]appentitlement.SubjectAssignment, 0, len(rows))
	for _, row := range rows {
		assignment, err := sqlitePlanSubjectAssignment(row)
		if err != nil {
			return nil, err
		}
		assignments = append(assignments, assignment)
	}
	return assignments, nil
}

func (r *EntitlementRepository) DeleteSubjectAssignment(ctx context.Context, subject string) error {
	workspaceID, err := appauth.RequireWorkspaceID(ctx)
	if err != nil {
		return err
	}
	now := formatTime(time.Now().UTC())
	rows, err := queriesFor(ctx, r.queries).DeletePlanSubjectAssignment(ctx, sqlitedb.DeletePlanSubjectAssignmentParams{
		UnassignedAt: sql.NullString{String: now, Valid: true},
		UpdatedAt:    now,
		WorkspaceID:  workspaceID,
		Subject:      subject,
	})
	if err != nil {
		return err
	}
	if rows == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *EntitlementRepository) GetEntitlementState(ctx context.Context, query appentitlement.StateQuery) (appentitlement.EntitlementState, error) {
	workspaceID, err := appauth.RequireWorkspaceID(ctx)
	if err != nil {
		return appentitlement.EntitlementState{}, err
	}
	row, err := queriesFor(ctx, r.queries).GetEntitlementState(ctx, sqlitedb.GetEntitlementStateParams{
		WorkspaceID: workspaceID,
		Subject:     query.Subject,
		MeterName:   query.MeterName,
		PlanID:      query.PlanID,
		Period:      string(query.Period),
	})
	if errors.Is(err, sql.ErrNoRows) {
		return appentitlement.EntitlementState{}, domain.ErrNotFound
	}
	if err != nil {
		return appentitlement.EntitlementState{}, err
	}
	return sqliteEntitlementState(row)
}

func (r *EntitlementRepository) FindEntitlementStates(ctx context.Context, query appentitlement.StateListQuery) ([]appentitlement.EntitlementState, error) {
	workspaceID, err := appauth.RequireWorkspaceID(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := queriesFor(ctx, r.queries).ListEntitlementStates(ctx, sqlitedb.ListEntitlementStatesParams{
		WorkspaceID: workspaceID,
		Subject:     planStringValue(query.Subject),
		MeterName:   planStringValue(query.MeterName),
		State:       planStringValue(string(query.State)),
		Limit:       int64(query.Limit),
	})
	if err != nil {
		return nil, err
	}
	states := make([]appentitlement.EntitlementState, 0, len(rows))
	for _, row := range rows {
		state, err := sqliteEntitlementState(row)
		if err != nil {
			return nil, err
		}
		states = append(states, state)
	}
	return states, nil
}

func (r *EntitlementRepository) FindEntitlementEvents(ctx context.Context, query appentitlement.EventQuery) ([]appentitlement.EntitlementEvent, error) {
	workspaceID, err := appauth.RequireWorkspaceID(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := queriesFor(ctx, r.queries).ListEntitlementEvents(ctx, sqlitedb.ListEntitlementEventsParams{
		WorkspaceID:     workspaceID,
		Subject:         planStringValue(query.Subject),
		MeterName:       planStringValue(query.MeterName),
		PlanID:          planStringValue(query.PlanID),
		State:           planStringValue(string(query.State)),
		Type:            planStringValue(string(query.Type)),
		CursorCreatedAt: entitlementTimeValue(query.CreatedAt),
		CursorID:        planStringValue(query.ID),
		Limit:           int64(query.Limit),
	})
	if err != nil {
		return nil, err
	}
	events := make([]appentitlement.EntitlementEvent, 0, len(rows))
	for _, row := range rows {
		event, err := sqliteEntitlementEvent(row)
		if err != nil {
			return nil, err
		}
		events = append(events, event)
	}
	return events, nil
}

func (r *EntitlementRepository) GetEntitlementUsageCounter(ctx context.Context, query appentitlement.CounterQuery) (appentitlement.EntitlementUsageCounter, error) {
	workspaceID, err := appauth.RequireWorkspaceID(ctx)
	if err != nil {
		return appentitlement.EntitlementUsageCounter{}, err
	}
	row, err := queriesFor(ctx, r.queries).GetEntitlementUsageCounter(ctx, sqlitedb.GetEntitlementUsageCounterParams{
		WorkspaceID: workspaceID,
		Subject:     query.Subject,
		MeterName:   query.MeterName,
		Period:      string(query.Period),
		PeriodStart: formatTime(query.From),
	})
	if errors.Is(err, sql.ErrNoRows) {
		return appentitlement.EntitlementUsageCounter{}, domain.ErrNotFound
	}
	if err != nil {
		return appentitlement.EntitlementUsageCounter{}, err
	}
	return sqliteEntitlementUsageCounter(row)
}

func (r *EntitlementRepository) SaveEntitlementState(ctx context.Context, state appentitlement.EntitlementState) error {
	workspaceID, err := appauth.RequireWorkspaceID(ctx)
	if err != nil {
		return err
	}
	return queriesFor(ctx, r.queries).SaveEntitlementState(ctx, sqlitedb.SaveEntitlementStateParams{
		WorkspaceID:    workspaceID,
		Subject:        state.Subject,
		MeterName:      state.MeterName,
		PlanID:         state.PlanID,
		PlanName:       state.PlanName,
		Period:         string(state.Period),
		State:          string(state.State),
		CurrentValue:   state.Current,
		LimitValue:     state.Limit,
		RemainingValue: state.Remaining,
		WarningPercent: state.WarningPercent,
		Message:        state.Message,
		EvaluatedAt:    formatTime(state.EvaluatedAt),
		UpdatedAt:      formatTime(state.UpdatedAt),
	})
}

func (r *EntitlementRepository) SaveEntitlementEvent(ctx context.Context, event appentitlement.EntitlementEvent) error {
	workspaceID, err := appauth.RequireWorkspaceID(ctx)
	if err != nil {
		return err
	}
	return queriesFor(ctx, r.queries).SaveEntitlementEvent(ctx, sqlitedb.SaveEntitlementEventParams{
		ID:             event.ID,
		WorkspaceID:    workspaceID,
		Subject:        event.Subject,
		MeterName:      event.MeterName,
		PlanID:         event.PlanID,
		PlanName:       event.PlanName,
		Period:         string(event.Period),
		PreviousState:  planStringValue(string(event.PreviousState)),
		State:          string(event.State),
		Type:           string(event.Type),
		CurrentValue:   event.Current,
		LimitValue:     event.Limit,
		RemainingValue: event.Remaining,
		WarningPercent: event.WarningPercent,
		Message:        event.Message,
		CreatedAt:      formatTime(event.CreatedAt),
	})
}

func (r *EntitlementRepository) EnqueueEntitlementCheckJob(ctx context.Context, job appentitlement.CheckJob) error {
	workspaceID, err := appauth.RequireWorkspaceID(ctx)
	if err != nil {
		return err
	}
	now := job.UpdatedAt
	if now.IsZero() {
		now = time.Now().UTC()
	}
	runAfter := job.RunAfter
	if runAfter.IsZero() {
		runAfter = now
	}
	return queriesFor(ctx, r.queries).EnqueueEntitlementCheckJob(ctx, sqlitedb.EnqueueEntitlementCheckJobParams{
		WorkspaceID: workspaceID,
		Subject:     job.Subject,
		MeterName:   job.MeterName,
		RunAfter:    formatTime(runAfter),
		Now:         formatTime(now),
	})
}

func (r *EntitlementRepository) ClaimEntitlementCheckJob(ctx context.Context, cmd appentitlement.ClaimCommand) (appentitlement.CheckJob, bool, error) {
	now := time.Now().UTC()
	row, err := queriesFor(ctx, r.queries).ClaimEntitlementCheckJob(ctx, sqlitedb.ClaimEntitlementCheckJobParams{
		LockedUntil: sql.NullString{String: formatTime(now.Add(cmd.LockTTL)), Valid: true},
		Now:         formatTime(now),
		MaxAttempts: int64(cmd.MaxAttempts),
	})
	if errors.Is(err, sql.ErrNoRows) {
		return appentitlement.CheckJob{}, false, nil
	}
	if err != nil {
		return appentitlement.CheckJob{}, false, err
	}
	job, err := sqliteEntitlementCheckJob(row)
	if err != nil {
		return appentitlement.CheckJob{}, false, err
	}
	return job, true, nil
}

func (r *EntitlementRepository) RequeueEntitlementCheckJob(ctx context.Context, cmd appentitlement.FailCommand) error {
	workspaceID, err := appauth.RequireWorkspaceID(ctx)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	rows, err := queriesFor(ctx, r.queries).RequeueEntitlementCheckJob(ctx, sqlitedb.RequeueEntitlementCheckJobParams{
		RunAfter:    formatTime(now.Add(cmd.RetryAfter)),
		Now:         formatTime(now),
		WorkspaceID: workspaceID,
		Subject:     cmd.Subject,
		MeterName:   cmd.Meter,
	})
	if err != nil {
		return err
	}
	if rows == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *EntitlementRepository) DeleteEntitlementCheckJob(ctx context.Context, cmd appentitlement.CompleteCommand) error {
	workspaceID, err := appauth.RequireWorkspaceID(ctx)
	if err != nil {
		return err
	}
	rows, err := queriesFor(ctx, r.queries).DeleteEntitlementCheckJob(ctx, sqlitedb.DeleteEntitlementCheckJobParams{
		WorkspaceID: workspaceID,
		Subject:     cmd.Subject,
		MeterName:   cmd.Meter,
	})
	if err != nil {
		return err
	}
	if rows == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func sqlitePlan(row sqlitedb.ListPlansRow) (appentitlement.Plan, error) {
	createdAt, err := parseEntitlementTime(row.CreatedAt)
	if err != nil {
		return appentitlement.Plan{}, err
	}
	updatedAt, err := parseEntitlementTime(row.UpdatedAt)
	if err != nil {
		return appentitlement.Plan{}, err
	}
	return appentitlement.Plan{
		ID:           row.ID,
		Name:         row.Name,
		Description:  row.Description,
		Version:      int(row.Version),
		ParentPlanID: row.ParentPlanID.String,
		IsCurrent:    row.IsCurrent != 0,
		CreatedAt:    createdAt,
		UpdatedAt:    updatedAt,
	}, nil
}

func sqlitePlanLimit(row sqlitedb.ListPlanLimitsRow) (appentitlement.PlanLimit, error) {
	createdAt, err := parseEntitlementTime(row.CreatedAt)
	if err != nil {
		return appentitlement.PlanLimit{}, err
	}
	updatedAt, err := parseEntitlementTime(row.UpdatedAt)
	if err != nil {
		return appentitlement.PlanLimit{}, err
	}
	return appentitlement.PlanLimit{
		ID:             row.ID,
		PlanID:         row.PlanID,
		MeterName:      row.MeterName,
		Period:         appentitlement.Period(row.Period),
		Limit:          row.LimitValue,
		WarningPercent: row.WarningPercent,
		CreatedAt:      createdAt,
		UpdatedAt:      updatedAt,
	}, nil
}

func sqlitePlanSubjectAssignment(row sqlitedb.ListPlanSubjectAssignmentsRow) (appentitlement.SubjectAssignment, error) {
	assignedAt, err := parseEntitlementTime(row.AssignedAt)
	if err != nil {
		return appentitlement.SubjectAssignment{}, err
	}
	periodAnchorAt, err := parseEntitlementTime(row.PeriodAnchorAt)
	if err != nil {
		return appentitlement.SubjectAssignment{}, err
	}
	updatedAt, err := parseEntitlementTime(row.UpdatedAt)
	if err != nil {
		return appentitlement.SubjectAssignment{}, err
	}
	unassignedAt, err := parseNullableEntitlementTime(row.UnassignedAt)
	if err != nil {
		return appentitlement.SubjectAssignment{}, err
	}
	return appentitlement.SubjectAssignment{
		ID:             row.ID,
		Subject:        row.Subject,
		PlanID:         row.PlanID,
		PlanName:       row.PlanName,
		PlanVersion:    int(row.PlanVersion),
		AssignedAt:     assignedAt,
		PeriodAnchorAt: periodAnchorAt,
		UnassignedAt:   unassignedAt,
		UpdatedAt:      updatedAt,
	}, nil
}

func sqliteEntitlementState(row sqlitedb.EntitlementState) (appentitlement.EntitlementState, error) {
	evaluatedAt, err := parseEntitlementTime(row.EvaluatedAt)
	if err != nil {
		return appentitlement.EntitlementState{}, err
	}
	updatedAt, err := parseEntitlementTime(row.UpdatedAt)
	if err != nil {
		return appentitlement.EntitlementState{}, err
	}
	return appentitlement.EntitlementState{
		WorkspaceID:    row.WorkspaceID,
		Subject:        row.Subject,
		MeterName:      row.MeterName,
		PlanID:         row.PlanID,
		PlanName:       row.PlanName,
		Period:         appentitlement.Period(row.Period),
		State:          appentitlement.OverageState(row.State),
		Current:        row.CurrentValue,
		Limit:          row.LimitValue,
		Remaining:      row.RemainingValue,
		WarningPercent: row.WarningPercent,
		Message:        row.Message,
		EvaluatedAt:    evaluatedAt,
		UpdatedAt:      updatedAt,
	}, nil
}

func sqliteEntitlementEvent(row sqlitedb.EntitlementEvent) (appentitlement.EntitlementEvent, error) {
	createdAt, err := parseEntitlementTime(row.CreatedAt)
	if err != nil {
		return appentitlement.EntitlementEvent{}, err
	}
	return appentitlement.EntitlementEvent{
		ID:             row.ID,
		WorkspaceID:    row.WorkspaceID,
		Subject:        row.Subject,
		MeterName:      row.MeterName,
		PlanID:         row.PlanID,
		PlanName:       row.PlanName,
		Period:         appentitlement.Period(row.Period),
		PreviousState:  appentitlement.OverageState(row.PreviousState.String),
		State:          appentitlement.OverageState(row.State),
		Type:           appentitlement.EventType(row.Type),
		Current:        row.CurrentValue,
		Limit:          row.LimitValue,
		Remaining:      row.RemainingValue,
		WarningPercent: row.WarningPercent,
		Message:        row.Message,
		CreatedAt:      createdAt,
	}, nil
}

func sqliteEntitlementUsageCounter(row sqlitedb.EntitlementUsageCounter) (appentitlement.EntitlementUsageCounter, error) {
	from, err := parseEntitlementTime(row.PeriodStart)
	if err != nil {
		return appentitlement.EntitlementUsageCounter{}, err
	}
	to, err := parseEntitlementTime(row.PeriodEnd)
	if err != nil {
		return appentitlement.EntitlementUsageCounter{}, err
	}
	firstEventTime, err := parseEntitlementTime(row.FirstEventTime)
	if err != nil {
		return appentitlement.EntitlementUsageCounter{}, err
	}
	lastEventTime, err := parseEntitlementTime(row.LastEventTime)
	if err != nil {
		return appentitlement.EntitlementUsageCounter{}, err
	}
	updatedAt, err := parseEntitlementTime(row.UpdatedAt)
	if err != nil {
		return appentitlement.EntitlementUsageCounter{}, err
	}
	return appentitlement.EntitlementUsageCounter{
		WorkspaceID:    row.WorkspaceID,
		Subject:        row.Subject,
		MeterName:      row.MeterName,
		Period:         appentitlement.Period(row.Period),
		From:           from,
		To:             to,
		EventCount:     row.EventCount,
		QuantitySum:    row.QuantitySum,
		QuantityMin:    row.QuantityMin,
		QuantityMax:    row.QuantityMax,
		FirstQuantity:  row.FirstQuantity,
		FirstEventTime: firstEventTime,
		LastQuantity:   row.LastQuantity,
		LastEventTime:  lastEventTime,
		UpdatedAt:      updatedAt,
	}, nil
}

func sqliteEntitlementCheckJob(row sqlitedb.EntitlementCheckJob) (appentitlement.CheckJob, error) {
	runAfter, err := parseEntitlementTime(row.RunAfter)
	if err != nil {
		return appentitlement.CheckJob{}, err
	}
	lockedUntil, err := parseNullableEntitlementTime(row.LockedUntil)
	if err != nil {
		return appentitlement.CheckJob{}, err
	}
	createdAt, err := parseEntitlementTime(row.CreatedAt)
	if err != nil {
		return appentitlement.CheckJob{}, err
	}
	updatedAt, err := parseEntitlementTime(row.UpdatedAt)
	if err != nil {
		return appentitlement.CheckJob{}, err
	}
	return appentitlement.CheckJob{
		WorkspaceID: row.WorkspaceID,
		Subject:     row.Subject,
		MeterName:   row.MeterName,
		RunAfter:    runAfter,
		LockedUntil: lockedUntil,
		Attempts:    int(row.Attempts),
		CreatedAt:   createdAt,
		UpdatedAt:   updatedAt,
	}, nil
}

func planStringValue(value string) sql.NullString {
	if value == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: value, Valid: true}
}

func entitlementTimeValue(value time.Time) sql.NullString {
	if value.IsZero() {
		return sql.NullString{}
	}
	return sql.NullString{String: formatTime(value), Valid: true}
}

func sqliteBool(value bool) int64 {
	if value {
		return 1
	}
	return 0
}

func parseEntitlementTime(value string) (time.Time, error) {
	return time.Parse(time.RFC3339Nano, value)
}

func parseNullableEntitlementTime(value sql.NullString) (time.Time, error) {
	if !value.Valid || value.String == "" {
		return time.Time{}, nil
	}
	return parseEntitlementTime(value.String)
}
