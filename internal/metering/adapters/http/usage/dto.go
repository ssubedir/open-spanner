package usage

type createRequest struct {
	IdempotencyKey string         `json:"idempotency_key"`
	Subject        string         `json:"subject"`
	Meter          string         `json:"meter"`
	Quantity       float64        `json:"quantity"`
	Timestamp      string         `json:"timestamp"`
	Metadata       map[string]any `json:"metadata"`
}

type response struct {
	ID             string         `json:"id"`
	IdempotencyKey string         `json:"idempotency_key,omitempty"`
	Subject        string         `json:"subject"`
	Meter          string         `json:"meter"`
	Quantity       float64        `json:"quantity"`
	Timestamp      string         `json:"timestamp"`
	ReceivedAt     string         `json:"received_at"`
	Metadata       map[string]any `json:"metadata"`
}

type listItemResponse struct {
	Subject     string  `json:"subject"`
	Meter       string  `json:"meter"`
	BucketSize  string  `json:"bucket_size"`
	BucketStart string  `json:"bucket_start"`
	Quantity    float64 `json:"quantity"`
}
