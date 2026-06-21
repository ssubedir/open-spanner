package entitlement

// LimitRequest defines a plan quota for a meter and period.
type LimitRequest struct {
	Meter          string  `json:"meter"`
	Period         string  `json:"period,omitempty"`
	Limit          float64 `json:"limit"`
	WarningPercent float64 `json:"warning_percent,omitempty"`
}

// PlanSaveRequest creates or replaces a plan.
type PlanSaveRequest struct {
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	Limits      []LimitRequest `json:"limits"`
}

// PlanResponse is a plan with its configured limits.
type PlanResponse struct {
	ID           string          `json:"id"`
	Name         string          `json:"name"`
	Description  string          `json:"description"`
	Version      int             `json:"version"`
	ParentPlanID string          `json:"parent_plan_id,omitempty"`
	IsCurrent    bool            `json:"is_current"`
	Limits       []LimitResponse `json:"limits"`
	CreatedAt    string          `json:"created_at"`
	UpdatedAt    string          `json:"updated_at"`
}

// LimitResponse is a plan limit.
type LimitResponse struct {
	ID             string  `json:"id"`
	Meter          string  `json:"meter"`
	Period         string  `json:"period"`
	Limit          float64 `json:"limit"`
	WarningPercent float64 `json:"warning_percent"`
	CreatedAt      string  `json:"created_at"`
	UpdatedAt      string  `json:"updated_at"`
}

// PlanListResponse is a list of plans.
type PlanListResponse struct {
	Items []PlanResponse `json:"items"`
}

// AssignmentRequest assigns a subject to a plan.
type AssignmentRequest struct {
	PlanID string `json:"plan_id"`
}

// AssignmentResponse is a subject plan assignment.
type AssignmentResponse struct {
	ID             string `json:"id"`
	Subject        string `json:"subject"`
	PlanID         string `json:"plan_id"`
	PlanName       string `json:"plan_name"`
	PlanVersion    int    `json:"plan_version"`
	Active         bool   `json:"active"`
	AssignedAt     string `json:"assigned_at"`
	PeriodAnchorAt string `json:"period_anchor_at"`
	UnassignedAt   string `json:"unassigned_at,omitempty"`
	UpdatedAt      string `json:"updated_at"`
}

// AssignmentListResponse is a list of subject assignments.
type AssignmentListResponse struct {
	Items []AssignmentResponse `json:"items"`
}

// ProgressResponse is current quota progress for a subject.
type ProgressResponse struct {
	Subject string                 `json:"subject"`
	Plan    PlanResponse           `json:"plan"`
	Items   []ProgressItemResponse `json:"items"`
}

// ProgressItemResponse is current usage against a plan limit.
type ProgressItemResponse struct {
	Meter          string  `json:"meter"`
	Period         string  `json:"period"`
	State          string  `json:"state"`
	Current        float64 `json:"current"`
	Limit          float64 `json:"limit"`
	Remaining      float64 `json:"remaining"`
	Percent        float64 `json:"percent"`
	WarningPercent float64 `json:"warning_percent"`
	From           string  `json:"from"`
	To             string  `json:"to"`
	Unit           string  `json:"unit"`
	Aggregation    string  `json:"aggregation"`
}

// CheckRequest checks whether a subject has quota for a meter.
type CheckRequest struct {
	Subject  string  `json:"subject"`
	Meter    string  `json:"meter"`
	Quantity float64 `json:"quantity,omitempty"`
}

// CheckResponse is the entitlement decision for a subject and meter.
type CheckResponse struct {
	Allowed   bool    `json:"allowed"`
	State     string  `json:"state"`
	Subject   string  `json:"subject"`
	Meter     string  `json:"meter"`
	Quantity  float64 `json:"quantity"`
	Current   float64 `json:"current"`
	Limit     float64 `json:"limit"`
	Remaining float64 `json:"remaining"`
	PlanID    string  `json:"plan_id,omitempty"`
	PlanName  string  `json:"plan_name,omitempty"`
	Period    string  `json:"period,omitempty"`
	From      string  `json:"from,omitempty"`
	To        string  `json:"to,omitempty"`
	Message   string  `json:"message"`
}

// StateResponse is the latest entitlement state for a subject and meter.
type StateResponse struct {
	Subject        string  `json:"subject"`
	Meter          string  `json:"meter"`
	PlanID         string  `json:"plan_id"`
	PlanName       string  `json:"plan_name"`
	Period         string  `json:"period"`
	State          string  `json:"state"`
	Current        float64 `json:"current"`
	Limit          float64 `json:"limit"`
	Remaining      float64 `json:"remaining"`
	WarningPercent float64 `json:"warning_percent"`
	Message        string  `json:"message"`
	EvaluatedAt    string  `json:"evaluated_at"`
	UpdatedAt      string  `json:"updated_at"`
}

// StateListResponse is a list of latest entitlement states.
type StateListResponse struct {
	Items []StateResponse `json:"items"`
}

// EventResponse is an entitlement state transition.
type EventResponse struct {
	ID             string  `json:"id"`
	Subject        string  `json:"subject"`
	Meter          string  `json:"meter"`
	PlanID         string  `json:"plan_id"`
	PlanName       string  `json:"plan_name"`
	Period         string  `json:"period"`
	PreviousState  string  `json:"previous_state,omitempty"`
	State          string  `json:"state"`
	Type           string  `json:"type"`
	Current        float64 `json:"current"`
	Limit          float64 `json:"limit"`
	Remaining      float64 `json:"remaining"`
	WarningPercent float64 `json:"warning_percent"`
	Message        string  `json:"message"`
	CreatedAt      string  `json:"created_at"`
}

// EventListResponse is a paginated list of entitlement events.
type EventListResponse struct {
	Items      []EventResponse `json:"items"`
	NextCursor string          `json:"next_cursor,omitempty"`
}

// PeriodSnapshotResponse is an auditable quota snapshot for a subject period.
type PeriodSnapshotResponse struct {
	Subject        string  `json:"subject"`
	Meter          string  `json:"meter"`
	PlanID         string  `json:"plan_id"`
	PlanName       string  `json:"plan_name"`
	PlanVersion    int     `json:"plan_version"`
	Period         string  `json:"period"`
	From           string  `json:"from"`
	To             string  `json:"to"`
	State          string  `json:"state"`
	Current        float64 `json:"current"`
	Limit          float64 `json:"limit"`
	Included       float64 `json:"included"`
	Overage        float64 `json:"overage"`
	Remaining      float64 `json:"remaining"`
	WarningPercent float64 `json:"warning_percent"`
	EventCount     int64   `json:"event_count"`
	UpdatedAt      string  `json:"updated_at"`
}

// PeriodSnapshotListResponse is a list of quota period snapshots.
type PeriodSnapshotListResponse struct {
	Items []PeriodSnapshotResponse `json:"items"`
}
