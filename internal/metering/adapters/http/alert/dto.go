package alert

type SaveRequest struct {
	Name                      string            `json:"name"`
	Meter                     string            `json:"meter"`
	Enabled                   *bool             `json:"enabled,omitempty"`
	Subject                   string            `json:"subject,omitempty"`
	Metadata                  map[string]string `json:"metadata,omitempty"`
	WindowSeconds             int               `json:"window_seconds,omitempty"`
	Comparator                string            `json:"comparator,omitempty"`
	Threshold                 float64           `json:"threshold"`
	EvaluationIntervalSeconds int               `json:"evaluation_interval_seconds,omitempty"`
	GroupBy                   string            `json:"group_by,omitempty"`
	DestinationID             string            `json:"destination_id"`
}

type UpdateRequest struct {
	Name                      *string            `json:"name,omitempty"`
	Meter                     *string            `json:"meter,omitempty"`
	Enabled                   *bool              `json:"enabled,omitempty"`
	Subject                   *string            `json:"subject,omitempty"`
	Metadata                  *map[string]string `json:"metadata,omitempty"`
	WindowSeconds             *int               `json:"window_seconds,omitempty"`
	Comparator                *string            `json:"comparator,omitempty"`
	Threshold                 *float64           `json:"threshold,omitempty"`
	EvaluationIntervalSeconds *int               `json:"evaluation_interval_seconds,omitempty"`
	GroupBy                   *string            `json:"group_by,omitempty"`
	DestinationID             *string            `json:"destination_id,omitempty"`
}

type DestinationSaveRequest struct {
	Name       string `json:"name"`
	Type       string `json:"type,omitempty"`
	Enabled    *bool  `json:"enabled,omitempty"`
	WebhookURL string `json:"webhook_url"`
}

type DestinationUpdateRequest struct {
	Name       *string `json:"name,omitempty"`
	Type       *string `json:"type,omitempty"`
	Enabled    *bool   `json:"enabled,omitempty"`
	WebhookURL *string `json:"webhook_url,omitempty"`
}

type RuleResponse struct {
	ID                        string               `json:"id"`
	Name                      string               `json:"name"`
	Meter                     string               `json:"meter"`
	Enabled                   bool                 `json:"enabled"`
	Subject                   string               `json:"subject,omitempty"`
	Metadata                  map[string]string    `json:"metadata,omitempty"`
	WindowSeconds             int                  `json:"window_seconds"`
	Comparator                string               `json:"comparator"`
	Threshold                 float64              `json:"threshold"`
	EvaluationIntervalSeconds int                  `json:"evaluation_interval_seconds"`
	GroupBy                   string               `json:"group_by,omitempty"`
	DestinationID             string               `json:"destination_id,omitempty"`
	Destination               *DestinationResponse `json:"destination,omitempty"`
	NextEvaluateAt            string               `json:"next_evaluate_at"`
	CreatedAt                 string               `json:"created_at"`
	UpdatedAt                 string               `json:"updated_at"`
	State                     *StateResponse       `json:"state,omitempty"`
	States                    []StateResponse      `json:"states,omitempty"`
}

type DestinationResponse struct {
	ID             string         `json:"id"`
	Name           string         `json:"name"`
	Type           string         `json:"type"`
	Enabled        bool           `json:"enabled"`
	WebhookURL     string         `json:"webhook_url"`
	WebhookSigning WebhookSigning `json:"webhook_signing"`
	CreatedAt      string         `json:"created_at"`
	UpdatedAt      string         `json:"updated_at"`
}

type WebhookSigning struct {
	Enabled         bool   `json:"enabled"`
	Algorithm       string `json:"algorithm"`
	SignatureHeader string `json:"signature_header"`
	TimestampHeader string `json:"timestamp_header"`
	Secret          string `json:"secret,omitempty"`
}

type StateResponse struct {
	Status      string  `json:"status"`
	GroupKey    string  `json:"group_key,omitempty"`
	GroupValue  string  `json:"group_value,omitempty"`
	Value       float64 `json:"value"`
	Message     string  `json:"message"`
	EvaluatedAt string  `json:"evaluated_at,omitempty"`
	UpdatedAt   string  `json:"updated_at"`
}

type EventResponse struct {
	ID         string            `json:"id"`
	RuleID     string            `json:"rule_id"`
	GroupKey   string            `json:"group_key,omitempty"`
	GroupValue string            `json:"group_value,omitempty"`
	Type       string            `json:"type"`
	Value      float64           `json:"value"`
	Message    string            `json:"message"`
	CreatedAt  string            `json:"created_at"`
	Delivery   *DeliveryResponse `json:"delivery,omitempty"`
}

type DeliveryResponse struct {
	ID          string `json:"id"`
	EventID     string `json:"event_id"`
	TriggerType string `json:"trigger_type"`
	Status      string `json:"status"`
	StatusCode  int    `json:"status_code,omitempty"`
	Error       string `json:"error,omitempty"`
	DurationMs  int    `json:"duration_ms"`
	AttemptedAt string `json:"attempted_at"`
	CreatedAt   string `json:"created_at"`
}

type RuleListResponse struct {
	Items []RuleResponse `json:"items"`
}

type DestinationListResponse struct {
	Items []DestinationResponse `json:"items"`
}

type EventListResponse struct {
	Items      []EventResponse `json:"items"`
	NextCursor string          `json:"next_cursor,omitempty"`
}

type EvaluationResponse struct {
	Rule   RuleResponse    `json:"rule"`
	State  StateResponse   `json:"state"`
	Event  *EventResponse  `json:"event,omitempty"`
	Events []EventResponse `json:"events,omitempty"`
}
