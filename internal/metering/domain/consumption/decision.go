package consumption

import (
	"encoding/json"
	"time"
)

type Decision struct {
	IdempotencyKey   string
	Snapshot         []byte
	Subject          string
	MeterName        string
	Accepted         bool
	EvaluationFailed bool
	Enforcement      string
	State            string
	CreatedAt        time.Time
}

type Query struct {
	Subject          string
	MeterName        string
	Accepted         *bool
	EvaluationFailed *bool
	Enforcement      string
	State            string
	CursorCreatedAt  time.Time
	CursorID         string
	Limit            int
}

func FromSnapshot(key string, snapshot []byte, createdAt time.Time) (Decision, error) {
	var indexed struct {
		Accepted         bool
		EvaluationFailed bool
		Quota            struct {
			Subject     string
			MeterName   string
			Enforcement string
			State       string
		}
	}
	if err := json.Unmarshal(snapshot, &indexed); err != nil {
		return Decision{}, err
	}
	return Decision{
		IdempotencyKey: key, Snapshot: snapshot, Subject: indexed.Quota.Subject,
		MeterName: indexed.Quota.MeterName, Accepted: indexed.Accepted,
		EvaluationFailed: indexed.EvaluationFailed, Enforcement: indexed.Quota.Enforcement,
		State: indexed.Quota.State, CreatedAt: createdAt,
	}, nil
}
