package usage

import (
	"fmt"
	"strings"
	"time"

	"open-spanner/internal/metering/domain"
)

type BucketSize string

const (
	BucketHour  BucketSize = "hour"
	BucketDay   BucketSize = "day"
	BucketMonth BucketSize = "month"
)

type Query struct {
	subject    string
	meterName  string
	from       time.Time
	to         time.Time
	bucketSize BucketSize
}

type Bucket struct {
	subject     string
	meterName   string
	bucketSize  BucketSize
	bucketStart time.Time
	quantity    float64
}

func NewQuery(subject, meterName string, from, to time.Time, bucketSize BucketSize) (Query, error) {
	subject = strings.TrimSpace(subject)
	meterName = strings.TrimSpace(meterName)

	if subject == "" {
		return Query{}, fmt.Errorf("%w: subject is required", domain.ErrInvalidInput)
	}
	if meterName == "" {
		return Query{}, fmt.Errorf("%w: meter is required", domain.ErrInvalidInput)
	}
	if from.IsZero() || to.IsZero() || !from.Before(to) {
		return Query{}, fmt.Errorf("%w: valid from and to range is required", domain.ErrInvalidInput)
	}
	if bucketSize == "" {
		bucketSize = BucketDay
	}
	switch bucketSize {
	case BucketHour, BucketDay, BucketMonth:
	default:
		return Query{}, fmt.Errorf("%w: unsupported bucket size %q", domain.ErrInvalidInput, bucketSize)
	}

	return Query{
		subject:    subject,
		meterName:  meterName,
		from:       from.UTC(),
		to:         to.UTC(),
		bucketSize: bucketSize,
	}, nil
}

func NewBucket(subject, meterName string, bucketSize BucketSize, bucketStart time.Time, quantity float64) Bucket {
	return Bucket{
		subject:     subject,
		meterName:   meterName,
		bucketSize:  bucketSize,
		bucketStart: bucketStart.UTC(),
		quantity:    quantity,
	}
}

func (q Query) Subject() string {
	return q.subject
}

func (q Query) MeterName() string {
	return q.meterName
}

func (q Query) From() time.Time {
	return q.from
}

func (q Query) To() time.Time {
	return q.to
}

func (q Query) BucketSize() BucketSize {
	return q.bucketSize
}

func (b Bucket) Subject() string {
	return b.subject
}

func (b Bucket) MeterName() string {
	return b.meterName
}

func (b Bucket) BucketSize() BucketSize {
	return b.bucketSize
}

func (b Bucket) BucketStart() time.Time {
	return b.bucketStart
}

func (b Bucket) Quantity() float64 {
	return b.quantity
}
