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
		ID:          plan.ID,
		WorkspaceID: workspaceID,
		Name:        plan.Name,
		Description: plan.Description,
		CreatedAt:   formatTime(plan.CreatedAt),
		UpdatedAt:   formatTime(plan.UpdatedAt),
	})
	if err != nil {
		if isUniqueConstraint(err) {
			return appentitlement.Plan{}, errors.Join(domain.ErrConflict, err)
		}
		return appentitlement.Plan{}, err
	}
	return plan, nil
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
	err = queriesFor(ctx, r.queries).SavePlanSubjectAssignment(ctx, sqlitedb.SavePlanSubjectAssignmentParams{
		WorkspaceID: workspaceID,
		Subject:     assignment.Subject,
		PlanID:      assignment.PlanID,
		AssignedAt:  formatTime(assignment.AssignedAt),
		UpdatedAt:   formatTime(assignment.UpdatedAt),
	})
	if err != nil {
		if isUniqueConstraint(err) {
			return appentitlement.SubjectAssignment{}, errors.Join(domain.ErrConflict, err)
		}
		return appentitlement.SubjectAssignment{}, err
	}
	assignments, err := r.FindSubjectAssignments(ctx, appentitlement.AssignmentQuery{Subject: assignment.Subject, Limit: 1})
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
	rows, err := queriesFor(ctx, r.queries).DeletePlanSubjectAssignment(ctx, sqlitedb.DeletePlanSubjectAssignmentParams{
		WorkspaceID: workspaceID,
		Subject:     subject,
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
		ID:          row.ID,
		Name:        row.Name,
		Description: row.Description,
		CreatedAt:   createdAt,
		UpdatedAt:   updatedAt,
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
	updatedAt, err := parseEntitlementTime(row.UpdatedAt)
	if err != nil {
		return appentitlement.SubjectAssignment{}, err
	}
	return appentitlement.SubjectAssignment{
		Subject:    row.Subject,
		PlanID:     row.PlanID,
		PlanName:   row.PlanName,
		AssignedAt: assignedAt,
		UpdatedAt:  updatedAt,
	}, nil
}

func planStringValue(value string) sql.NullString {
	if value == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: value, Valid: true}
}

func parseEntitlementTime(value string) (time.Time, error) {
	return time.Parse(time.RFC3339Nano, value)
}
