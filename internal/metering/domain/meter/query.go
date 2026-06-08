package meter

const (
	DefaultLimit = 50
	MaxLimit     = 500
)

type Query struct {
	ID     string
	Name   string
	Cursor string
	Limit  int
}

func NormalizeLimit(limit int) int {
	if limit <= 0 {
		return DefaultLimit
	}
	if limit > MaxLimit {
		return MaxLimit
	}
	return limit
}
