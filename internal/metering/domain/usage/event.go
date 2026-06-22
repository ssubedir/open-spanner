package usage

import (
	"fmt"
	"strings"
	"time"

	"github.com/ssubedir/open-spanner/internal/metering/domain"
)

type Event struct {
	id             string
	idempotencyKey string
	subject        string
	meterName      string
	quantity       float64
	eventTime      time.Time
	receivedAt     time.Time
	metadata       map[string]any
}

func NewEvent(
	id string,
	idempotencyKey string,
	subject string,
	meterName string,
	quantity float64,
	eventTime time.Time,
	receivedAt time.Time,
	metadata map[string]any,
) (Event, error) {
	id = strings.TrimSpace(id)
	idempotencyKey = strings.TrimSpace(idempotencyKey)
	meterName = strings.TrimSpace(meterName)
	var err error
	subject, err = NormalizeSubject(subject)

	if id == "" {
		return Event{}, fmt.Errorf("%w: event id is required", domain.ErrInvalidInput)
	}
	if err != nil {
		return Event{}, err
	}
	if meterName == "" {
		return Event{}, fmt.Errorf("%w: meter is required", domain.ErrInvalidInput)
	}
	if quantity <= 0 {
		return Event{}, fmt.Errorf("%w: quantity must be greater than zero", domain.ErrInvalidInput)
	}
	if eventTime.IsZero() {
		return Event{}, fmt.Errorf("%w: event time is required", domain.ErrInvalidInput)
	}
	if receivedAt.IsZero() {
		return Event{}, fmt.Errorf("%w: received at is required", domain.ErrInvalidInput)
	}
	if metadata == nil {
		metadata = map[string]any{}
	}

	return Event{
		id:             id,
		idempotencyKey: idempotencyKey,
		subject:        subject,
		meterName:      meterName,
		quantity:       quantity,
		eventTime:      eventTime.UTC(),
		receivedAt:     receivedAt.UTC(),
		metadata:       metadata,
	}, nil
}

func (e Event) ID() string {
	return e.id
}

func (e Event) IdempotencyKey() string {
	return e.idempotencyKey
}

func (e Event) Subject() string {
	return e.subject
}

func (e Event) MeterName() string {
	return e.meterName
}

func (e Event) Quantity() float64 {
	return e.quantity
}

func (e Event) EventTime() time.Time {
	return e.eventTime
}

func (e Event) ReceivedAt() time.Time {
	return e.receivedAt
}

func (e Event) Metadata() map[string]any {
	return e.metadata
}
