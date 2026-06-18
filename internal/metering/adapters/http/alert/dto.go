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
	TriggerType               string            `json:"trigger_type,omitempty"`
	WebhookURL                string            `json:"webhook_url,omitempty"`
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
	TriggerType               *string            `json:"trigger_type,omitempty"`
	WebhookURL                *string            `json:"webhook_url,omitempty"`
}

type RuleResponse struct {
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
	TriggerType               string            `json:"trigger_type"`
	WebhookURL                string            `json:"webhook_url,omitempty"`
	NextEvaluateAt            string            `json:"next_evaluate_at"`
	CreatedAt                 string            `json:"created_at"`
	UpdatedAt                 string            `json:"updated_at"`
	State                     *StateResponse    `json:"state,omitempty"`
	States                    []StateResponse   `json:"states,omitempty"`
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
	ID         string  `json:"id"`
	RuleID     string  `json:"rule_id"`
	GroupKey   string  `json:"group_key,omitempty"`
	GroupValue string  `json:"group_value,omitempty"`
	Type       string  `json:"type"`
	Value      float64 `json:"value"`
	Message    string  `json:"message"`
	CreatedAt  string  `json:"created_at"`
}

type RuleListResponse struct {
	Items []RuleResponse `json:"items"`
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
