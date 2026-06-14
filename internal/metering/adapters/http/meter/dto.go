package meter

// CreateRequest creates a meter.
type CreateRequest struct {
	Name               string            `json:"name"`
	Description        string            `json:"description"`
	Unit               string            `json:"unit"`
	Aggregation        string            `json:"aggregation"`
	MetadataSchema     map[string]string `json:"metadata_schema"`
	EventRetentionDays int               `json:"event_retention_days"`
}

// UpdateRequest updates a meter.
type UpdateRequest struct {
	Description        *string            `json:"description,omitempty"`
	Unit               *string            `json:"unit,omitempty"`
	Aggregation        *string            `json:"aggregation,omitempty"`
	MetadataSchema     *map[string]string `json:"metadata_schema,omitempty"`
	EventRetentionDays *int               `json:"event_retention_days,omitempty"`
}

// Response is a meter.
type Response struct {
	ID                 string            `json:"id"`
	Name               string            `json:"name"`
	Description        string            `json:"description"`
	Unit               string            `json:"unit"`
	Aggregation        string            `json:"aggregation"`
	MetadataSchema     map[string]string `json:"metadata_schema"`
	EventRetentionDays int               `json:"event_retention_days"`
	CreatedAt          string            `json:"created_at"`
}

// StatsResponse is meter activity stats.
type StatsResponse struct {
	Meter              string `json:"meter"`
	UsageEvents        int    `json:"usage_events"`
	LastEventAt        string `json:"last_event_at,omitempty"`
	EventRetentionDays int    `json:"retention_days"`
}

// ListResponse is a paged meter list.
type ListResponse struct {
	Items      []Response `json:"items"`
	NextCursor string     `json:"next_cursor,omitempty"`
}

// StatsListResponse is a paged meter stats list.
type StatsListResponse struct {
	Items      []StatsResponse `json:"items"`
	NextCursor string          `json:"next_cursor,omitempty"`
}
