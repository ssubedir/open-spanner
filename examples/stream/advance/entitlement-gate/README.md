# Entitlement Gate Stream Example

Checks quota through the REST entitlement endpoint, then emits accepted usage
through the gRPC stream client. This is useful for high-throughput services that
need a fast stream ingestion path but still want to guard work with plan limits.

Create this meter and assign a subject to a plan before running the example:

```text
name: stream_api_calls
unit: call
aggregation: sum
dimensions:
  endpoint: string
  region: string
  source: string
```

The API key needs `plans:read` for the entitlement check and `usage:write` for
stream ingestion.

Run:

```sh
cd examples/stream/advance/entitlement-gate
OPEN_SPANNER_API_KEY=osp_... go run .
```

Optional settings:

```sh
OPEN_SPANNER_BASE_URL=http://localhost:18081
OPEN_SPANNER_GRPC_ADDR=localhost:18090
OPEN_SPANNER_GRPC_METER=stream_api_calls
OPEN_SPANNER_SUBJECT=org_123
OPEN_SPANNER_QUANTITY=1
```

