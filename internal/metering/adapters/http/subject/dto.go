package subject

// Response is subject activity stats.
type Response struct {
	Subject     string `json:"subject"`
	UsageEvents int    `json:"usage_events"`
	Meters      int    `json:"meters"`
	LastEventAt string `json:"last_event_at"`
}

// ListResponse is a paged subject stats list.
type ListResponse struct {
	Items      []Response `json:"items"`
	NextCursor string     `json:"next_cursor,omitempty"`
}

// EventResponse is a subject usage event.
type EventResponse struct {
	ID             string         `json:"id"`
	IdempotencyKey string         `json:"idempotency_key,omitempty"`
	Subject        string         `json:"subject"`
	Meter          string         `json:"meter"`
	Quantity       float64        `json:"quantity"`
	Timestamp      string         `json:"timestamp"`
	ReceivedAt     string         `json:"received_at"`
	Metadata       map[string]any `json:"metadata"`
}
