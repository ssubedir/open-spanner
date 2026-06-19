package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/ssubedir/open-spanner/sdk/go/stream"
)

type sessionSample struct {
	Subject       string
	Protocol      string
	Region        string
	ClientVersion string
	Plan          string
	CloseReason   string
	ConnectedSecs float64
}

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	client, err := stream.NewClient(env("OPEN_SPANNER_GRPC_ADDR", "localhost:18090"), mustEnv("OPEN_SPANNER_API_KEY"))
	if err != nil {
		panic(err)
	}
	defer client.Close()

	meterName := env("OPEN_SPANNER_GRPC_METER", "stream_realtime_session_seconds")
	now := time.Now().UTC()
	samples := []sessionSample{
		{Subject: "org_acme", Protocol: "websocket", Region: "us-east", ClientVersion: "web-4.8.1", Plan: "enterprise", CloseReason: "normal", ConnectedSecs: 1820},
		{Subject: "org_acme", Protocol: "websocket", Region: "us-east", ClientVersion: "web-4.8.1", Plan: "enterprise", CloseReason: "timeout", ConnectedSecs: 420},
		{Subject: "org_globex", Protocol: "grpc", Region: "eu-west", ClientVersion: "agent-2.3.0", Plan: "pro", CloseReason: "normal", ConnectedSecs: 2475},
	}

	usageStream, err := client.Stream(ctx, "stream-websocket-sessions-"+fmt.Sprint(now.UnixNano()))
	if err != nil {
		panic(err)
	}
	for index, sample := range samples {
		err := usageStream.Track(stream.Event{
			IdempotencyKey: fmt.Sprintf("websocket-session-%d-%d", now.UnixNano(), index),
			Subject:        sample.Subject,
			Meter:          meterName,
			Quantity:       sample.ConnectedSecs,
			Timestamp:      now.Add(time.Duration(index) * time.Second),
			Metadata: map[string]any{
				"protocol":       sample.Protocol,
				"region":         sample.Region,
				"client_version": sample.ClientVersion,
				"plan":           sample.Plan,
				"close_reason":   sample.CloseReason,
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
