# Stream Examples

These examples focus on usage ingestion over gRPC. Use the dashboard or REST API
to create meters and API keys first, then use the stream SDK client from trusted
backend code.

| SDK | Command |
| --- | --- |
| Go | `cd examples/stream/basic/go && OPEN_SPANNER_API_KEY=osp_... go run main.go` |

## Advanced Use Cases

Each advanced stream folder is a standalone Go project:

| Scenario | Command |
| --- | --- |
| Device telemetry | `cd examples/stream/advance/device-telemetry && OPEN_SPANNER_API_KEY=osp_... go run .` |
| WebSocket sessions | `cd examples/stream/advance/websocket-sessions && OPEN_SPANNER_API_KEY=osp_... go run .` |
| Queue consumer | `cd examples/stream/advance/queue-consumer && OPEN_SPANNER_API_KEY=osp_... go run .` |
| Entitlement gate | `cd examples/stream/advance/entitlement-gate && OPEN_SPANNER_API_KEY=osp_... go run .` |
