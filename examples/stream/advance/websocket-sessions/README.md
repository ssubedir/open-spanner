# WebSocket Session Stream Example

Streams real-time connection session usage from a WebSocket or gRPC gateway.
This is useful when connected seconds or realtime presence is the billable unit.

Create this meter before running the example:

```text
name: stream_realtime_session_seconds
unit: second
aggregation: sum
dimensions:
  protocol: string
  region: string
  client_version: string
  plan: string
  close_reason: string
```

Run:

```sh
cd examples/stream/advance/websocket-sessions
OPEN_SPANNER_API_KEY=osp_... go run .
```

Optional settings:

```sh
OPEN_SPANNER_GRPC_ADDR=localhost:18090
OPEN_SPANNER_GRPC_METER=stream_realtime_session_seconds
```
