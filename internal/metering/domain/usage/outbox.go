package usage

import "time"

type OutboxMessage struct {
	ID          string
	WorkspaceID string
	EventID     string
	Subject     string
	MeterName   string
	Quantity    float64
	Metadata    map[string]any
	Attempts    int
	ClaimToken  string
	CreatedAt   time.Time
}
