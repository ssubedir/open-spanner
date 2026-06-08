package meter

type createRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Unit        string `json:"unit"`
	Aggregation string `json:"aggregation"`
}

type response struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Unit        string `json:"unit"`
	Aggregation string `json:"aggregation"`
	CreatedAt   string `json:"created_at"`
}
