package savedquery

import "encoding/json"

type SaveRequest struct {
	Name       string          `json:"name"`
	Query      json.RawMessage `json:"query"`
	GroupBy    []string        `json:"group_by"`
	BucketSize string          `json:"bucket_size"`
	Limit      int             `json:"limit"`
}

type Response struct {
	ID         string          `json:"id"`
	Name       string          `json:"name"`
	Query      json.RawMessage `json:"query"`
	GroupBy    []string        `json:"group_by"`
	BucketSize string          `json:"bucket_size"`
	Limit      int             `json:"limit"`
	CreatedAt  string          `json:"created_at"`
	UpdatedAt  string          `json:"updated_at"`
}

type ListResponse struct {
	Items []Response `json:"items"`
}
