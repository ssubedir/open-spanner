package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/ssubedir/open-spanner/sdk/go/stream"
)

type queueSample struct {
	Subject       string
	Queue         string
	Topic         string
	ConsumerGroup string
	Partition     int
	Outcome       string
	Region        string
	Messages      float64
}

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	client, err := stream.NewClient(env("OPEN_SPANNER_GRPC_ADDR", "localhost:18090"), mustEnv("OPEN_SPANNER_API_KEY"))
	if err != nil {
		panic(err)
	}
	defer client.Close()

	meterName := env("OPEN_SPANNER_GRPC_METER", "stream_queue_messages")
	now := time.Now().UTC()
	samples := []queueSample{
		{Subject: "org_acme", Queue: "billing-events", Topic: "usage.created", ConsumerGroup: "metering-writer", Partition: 3, Outcome: "acked", Region: "us-east", Messages: 250},
		{Subject: "org_acme", Queue: "billing-events", Topic: "usage.created", ConsumerGroup: "metering-writer", Partition: 4, Outcome: "acked", Region: "us-east", Messages: 175},
		{Subject: "org_globex", Queue: "audit-events", Topic: "session.closed", ConsumerGroup: "audit-exporter", Partition: 1, Outcome: "retried", Region: "eu-west", Messages: 12},
	}

	usageStream, err := client.Stream(ctx, "stream-queue-consumer-"+fmt.Sprint(now.UnixNano()))
	if err != nil {
		panic(err)
	}
	for index, sample := range samples {
		err := usageStream.Track(stream.Event{
			IdempotencyKey: fmt.Sprintf("queue-consumer-%d-%d", now.UnixNano(), index),
			Subject:        sample.Subject,
			Meter:          meterName,
			Quantity:       sample.Messages,
			Timestamp:      now.Add(time.Duration(index) * time.Second),
			Metadata: map[string]any{
				"queue":          sample.Queue,
				"topic":          sample.Topic,
				"consumer_group": sample.ConsumerGroup,
				"partition":      sample.Partition,
				"outcome":        sample.Outcome,
				"region":         sample.Region,
			},
		})
		if err != nil {
			panic(err)
		}
	}

	result, err := usageStream.Close()
	if err != nil {
		panic(err)
	}
	fmt.Printf("meter=%s accepted=%d duplicates=%d failed=%d\n", meterName, result.AcceptedCount, result.DuplicateCount, result.FailedCount)
}

func env(key string, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
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
