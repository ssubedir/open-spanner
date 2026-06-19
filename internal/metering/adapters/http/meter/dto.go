package meter

// CreateRequest creates a meter.
type CreateRequest struct {
	Name               string             `json:"name"`
	Description        string             `json:"description"`
	Unit               string             `json:"unit"`
	Aggregation        string             `json:"aggregation"`
	Dimensions         []DimensionRequest `json:"dimensions,omitempty"`
	EventRetentionDays int                `json:"event_retention_days"`
}

// UpdateRequest updates a meter.
type UpdateRequest struct {
	Description        *string             `json:"description,omitempty"`
	Unit               *string             `json:"unit,omitempty"`
	Aggregation        *string             `json:"aggregation,omitempty"`
	Dimensions         *[]DimensionRequest `json:"dimensions,omitempty"`
	EventRetentionDays *int                `json:"event_retention_days,omitempty"`
}

// DimensionRequest defines a meter dimension.
type DimensionRequest struct {
	Name        string `json:"name"`
	DisplayName string `json:"display_name,omitempty"`
	Description string `json:"description,omitempty"`
	Type        string `json:"type"`
	Required    *bool  `json:"required,omitempty"`
	Deprecated  bool   `json:"deprecated,omitempty"`
}

// Response is a meter.
type Response struct {
	ID                 string              `json:"id"`
	Name               string              `json:"name"`
	Description        string              `json:"description"`
	Unit               string              `json:"unit"`
	Aggregation        string              `json:"aggregation"`
	Dimensions         []DimensionResponse `json:"dimensions"`
	EventRetentionDays int                 `json:"event_retention_days"`
	CreatedAt          string              `json:"created_at"`
}

// DimensionResponse is a meter dimension.
type DimensionResponse struct {
	Name        string `json:"name"`
	DisplayName string `json:"display_name"`
	Description string `json:"description"`
	Type        string `json:"type"`
	Required    bool   `json:"required"`
	Deprecated  bool   `json:"deprecated"`
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
