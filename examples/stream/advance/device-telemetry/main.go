package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/ssubedir/open-spanner/sdk/go/stream"
)

type deviceSample struct {
	Subject    string
	DeviceType string
	Firmware   string
	Gateway    string
	Region     string
	Signal     string
	EnergyWh   float64
}

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	client, err := stream.NewClient(env("OPEN_SPANNER_GRPC_ADDR", "localhost:18090"), mustEnv("OPEN_SPANNER_API_KEY"))
	if err != nil {
		panic(err)
	}
	defer client.Close()

	meterName := env("OPEN_SPANNER_GRPC_METER", "stream_device_energy_wh")
	now := time.Now().UTC()
	samples := []deviceSample{
		{Subject: "fleet_north", DeviceType: "meter", Firmware: "2026.06.1", Gateway: "gw-a12", Region: "us-east", Signal: "lte", EnergyWh: 42.7},
		{Subject: "fleet_north", DeviceType: "meter", Firmware: "2026.06.1", Gateway: "gw-a12", Region: "us-east", Signal: "lte", EnergyWh: 38.4},
		{Subject: "fleet_west", DeviceType: "sensor", Firmware: "2026.05.4", Gateway: "gw-c08", Region: "us-west", Signal: "wifi", EnergyWh: 11.2},
		{Subject: "fleet_west", DeviceType: "meter", Firmware: "2026.06.1", Gateway: "gw-c08", Region: "us-west", Signal: "wifi", EnergyWh: 47.9},
	}

	usageStream, err := client.Stream(ctx, "stream-device-telemetry-"+fmt.Sprint(now.UnixNano()))
	if err != nil {
		panic(err)
	}
	for index, sample := range samples {
		err := usageStream.Track(stream.Event{
			IdempotencyKey: fmt.Sprintf("device-telemetry-%d-%d", now.UnixNano(), index),
			Subject:        sample.Subject,
			Meter:          meterName,
			Quantity:       sample.EnergyWh,
			Timestamp:      now.Add(time.Duration(index) * time.Second),
			Metadata: map[string]any{
				"device_type": sample.DeviceType,
				"firmware":    sample.Firmware,
				"gateway":     sample.Gateway,
				"region":      sample.Region,
				"signal":      sample.Signal,
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
