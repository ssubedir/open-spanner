package usage

import (
	"encoding/json"
	"fmt"

	domainusage "github.com/ssubedir/open-spanner/internal/metering/domain/usage"
)

// CreateRequest creates a usage event. IdempotencyKey replays a previously accepted event with the same key.
type CreateRequest struct {
	// IdempotencyKey replays the original accepted event when reused.
	IdempotencyKey string         `json:"idempotency_key"`
	Subject        string         `json:"subject"`
	Meter          string         `json:"meter"`
	Quantity       float64        `json:"quantity"`
	Timestamp      string         `json:"timestamp"`
	Metadata       map[string]any `json:"metadata"`
}

// FilterRequest is an advanced usage search filter.
type FilterRequest struct {
	Type  string          `json:"type"`
	Op    string          `json:"op"`
	Rules []FilterRequest `json:"rules,omitempty"`
	Field string          `json:"field,omitempty"`
	Value any             `json:"value,omitempty"`
}

// SearchRequest searches bucketed usage with an advanced filter.
type SearchRequest struct {
	Subject    string         `json:"subject"`
	Meter      string         `json:"meter"`
	From       string         `json:"from"`
	To         string         `json:"to"`
	BucketSize string         `json:"bucket_size"`
	GroupBy    GroupByRequest `json:"group_by,omitempty" swaggertype:"array,string"`
	Limit      int            `json:"limit,omitempty"`
	Filter     *FilterRequest `json:"filter,omitempty"`
}

// GroupByRequest accepts a single metadata key or an ordered list of metadata keys.
type GroupByRequest []string

func (g *GroupByRequest) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		*g = nil
		return nil
	}

	var values []string
	if err := json.Unmarshal(data, &values); err == nil {
		*g = GroupByRequest(domainusage.SplitGroupByValues(values))
		return nil
	}

	var value string
	if err := json.Unmarshal(data, &value); err == nil {
		*g = GroupByRequest(domainusage.SplitGroupBy(value))
		return nil
	}

	return fmt.Errorf("group_by must be a string or array of strings")
}

func (g GroupByRequest) Fields() []string {
	fields := make([]string, len(g))
	copy(fields, g)
	return fields
}

// EventSearchRequest searches raw usage events with an advanced filter.
type EventSearchRequest struct {
	Subject string         `json:"subject,omitempty"`
	Meter   string         `json:"meter,omitempty"`
	From    string         `json:"from,omitempty"`
	To      string         `json:"to,omitempty"`
	Limit   int            `json:"limit,omitempty"`
	Cursor  string         `json:"cursor,omitempty"`
	Filter  *FilterRequest `json:"filter,omitempty"`
}

// Response is a usage event.
type Response struct {
	ID             string         `json:"id"`
	IdempotencyKey string         `json:"idempotency_key,omitempty"`
	Subject        string         `json:"subject"`
	Meter          string         `json:"meter"`
	Quantity       float64        `json:"quantity"`
	Timestamp      string         `json:"timestamp"`
	ReceivedAt     string         `json:"received_at"`
	Metadata       map[string]any `json:"metadata"`
}

// EventListResponse is a paged raw usage event list.
type EventListResponse struct {
	Items      []Response `json:"items"`
	NextCursor string     `json:"next_cursor,omitempty"`
}

// PruneListResponse is a paged prune run list.
type PruneListResponse struct {
	Items      []PruneResponse `json:"items"`
	NextCursor string          `json:"next_cursor,omitempty"`
}

// IngestionListResponse is a paged ingestion run list.
type IngestionListResponse struct {
	Items      []IngestionResponse `json:"items"`
	NextCursor string              `json:"next_cursor,omitempty"`
}

// BulkResponse is a bulk ingestion result.
type BulkResponse struct {
	AcceptedCount  int                   `json:"accepted"`
	DuplicateCount int                   `json:"duplicates"`
	FailedCount    int                   `json:"failed"`
	Accepted       []Response            `json:"accepted_items"`
	Duplicates     []Response            `json:"duplicate_items"`
	Failed         []BulkFailureResponse `json:"failed_items"`
}

// BulkFailureResponse is a failed bulk item.
type BulkFailureResponse struct {
	Index   int    `json:"index"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

// PruneResponse is a prune run.
type PruneResponse struct {
	ID        string               `json:"id"`
	Deleted   int                  `json:"deleted"`
	DryRun    bool                 `json:"dry_run"`
	Meters    []PruneMeterResponse `json:"meters"`
	CreatedAt string               `json:"created_at"`
}

// PruneMeterResponse is a per-meter prune result.
type PruneMeterResponse struct {
	Meter   string `json:"meter"`
	Before  string `json:"before"`
	Deleted int    `json:"deleted"`
}

// IngestionResponse is an ingestion run summary.
type IngestionResponse struct {
	ID         string `json:"id"`
	Kind       string `json:"kind"`
	Accepted   int    `json:"accepted"`
	Duplicates int    `json:"duplicates"`
	Failed     int    `json:"failed"`
	CreatedAt  string `json:"created_at"`
}

// ListItemResponse is a usage bucket.
type ListItemResponse struct {
	Subject     string            `json:"subject"`
	Meter       string            `json:"meter"`
	BucketSize  string            `json:"bucket_size"`
	BucketStart string            `json:"bucket_start"`
	Aggregation string            `json:"aggregation"`
	Unit        string            `json:"unit"`
	Quantity    float64           `json:"quantity"`
	Group       map[string]string `json:"group,omitempty"`
}
