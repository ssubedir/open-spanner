package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/ssubedir/open-spanner/sdk/go/stream"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	apiKey := mustEnv("OPEN_SPANNER_API_KEY")
	grpcAddr := env("OPEN_SPANNER_GRPC_ADDR", "localhost:18090")
	meterName := env("OPEN_SPANNER_GRPC_METER", "grpc_go_requests")

	now := time.Now().UTC()
	subject := "org_grpc_go"

	client, err := stream.NewClient(grpcAddr, apiKey)
	if err != nil {
		panic(err)
	}
	defer client.Close()

	bulk, err := client.TrackBulk(ctx, "grpc-go-bulk-"+meterName, []stream.Event{
		usageEvent("bulk-1-"+meterName, subject, meterName, 2, now, map[string]any{"endpoint": "/orders", "status": 200}),
		usageEvent("bulk-2-"+meterName, subject, meterName, 3, now.Add(time.Second), map[string]any{"endpoint": "/users", "status": 201}),
	})
	if err != nil {
		panic(err)
	}

	usageStream, err := client.Stream(ctx, "grpc-go-stream-"+meterName)
	if err != nil {
		panic(err)
	}
	if err := usageStream.Track(usageEvent("stream-1-"+meterName, subject, meterName, 7, now.Add(2*time.Second), map[string]any{"endpoint": "/checkout", "status": 200})); err != nil {
		panic(err)
	}
	streamed, err := usageStream.Close()
	if err != nil {
		panic(err)
	}

	fmt.Printf("meter: %s\n", meterName)
	fmt.Printf("bulk accepted=%d duplicates=%d failed=%d\n", bulk.AcceptedCount, bulk.DuplicateCount, bulk.FailedCount)
	fmt.Printf("stream accepted=%d duplicates=%d failed=%d\n", streamed.AcceptedCount, streamed.DuplicateCount, streamed.FailedCount)
}

func usageEvent(idempotencyKey string, subject string, meterName string, quantity float64, eventTime time.Time, fields map[string]any) stream.Event {
	return stream.Event{
		IdempotencyKey: idempotencyKey,
		Subject:        subject,
		Meter:          meterName,
		Quantity:       quantity,
		Timestamp:      eventTime,
		Metadata:       fields,
	}
}

func env(key string, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}

func mustEnv(key string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		panic(key + " is required")
	}
	return value
}
