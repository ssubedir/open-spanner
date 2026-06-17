package usage

import (
	"encoding/csv"
	"encoding/json"
	"io"
	"strconv"
	"time"

	domainusage "github.com/ssubedir/open-spanner/internal/metering/domain/usage"
)

func WriteBucketCSV(w io.Writer, groupBy []string, buckets []ListItemResult) error {
	writer := csv.NewWriter(w)
	header := []string{"bucket_start", "subject", "meter", "bucket_size", "aggregation", "unit", "quantity"}
	if len(groupBy) > 0 {
		for _, field := range groupBy {
			if domainusage.IsSubjectGroupBy(field) {
				continue
			}
			header = append(header, field)
		}
	}
	if err := writer.Write(header); err != nil {
		return err
	}
	for _, bucket := range buckets {
		row := []string{
			bucket.BucketStart.Format(time.RFC3339),
			bucket.Subject,
			bucket.MeterName,
			bucket.BucketSize,
			bucket.Aggregation,
			bucket.Unit,
			strconv.FormatFloat(bucket.Quantity, 'f', -1, 64),
		}
		for _, field := range groupBy {
			if domainusage.IsSubjectGroupBy(field) {
				continue
			}
			row = append(row, bucket.Group[field])
		}
		if err := writer.Write(row); err != nil {
			return err
		}
	}
	writer.Flush()
	return writer.Error()
}

func WriteEventCSV(w io.Writer, events []Result) error {
	writer := csv.NewWriter(w)
	if err := writer.Write([]string{"timestamp", "received_at", "subject", "meter", "quantity", "metadata", "id", "idempotency_key"}); err != nil {
		return err
	}
	for _, event := range events {
		metadata, err := json.Marshal(event.Metadata)
		if err != nil {
			metadata = []byte("{}")
		}
		if err := writer.Write([]string{
			event.EventTime.Format(time.RFC3339),
			event.ReceivedAt.Format(time.RFC3339),
			event.Subject,
			event.MeterName,
			strconv.FormatFloat(event.Quantity, 'f', -1, 64),
			string(metadata),
			event.ID,
			event.IdempotencyKey,
		}); err != nil {
			return err
		}
	}
	writer.Flush()
	return writer.Error()
}
