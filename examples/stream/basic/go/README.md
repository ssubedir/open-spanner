# Go gRPC Basic Example

This example writes usage through the Go SDK's gRPC ingestion client.

Create a meter before running the example:

```text
name: grpc_go_requests
unit: request
aggregation: sum
dimensions:
  endpoint: string
  status: number
```

The wrapper keeps the sample focused on product behavior:

```go
client, _ := stream.NewClient("localhost:18090", apiKey)
defer client.Close()

client.TrackBulk(ctx, "batch-1", []stream.Event{event})

usageStream, _ := client.Stream(ctx, "stream-1")
usageStream.Track(event)
usageStream.Close()
```

Start Open Spanner with both REST and gRPC enabled:

```sh
task run:sqlite
```

Create an API key in the dashboard, then run:

```sh
cd examples/stream/basic/go
OPEN_SPANNER_API_KEY=osp_... go run main.go
```

Optional settings:

```sh
OPEN_SPANNER_GRPC_ADDR=localhost:18090
OPEN_SPANNER_GRPC_METER=grpc_go_requests
```
