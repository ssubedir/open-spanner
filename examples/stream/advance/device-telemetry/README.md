# Device Telemetry Stream Example

Streams high-frequency device telemetry from gateways. This is a good fit for
gRPC because devices or edge collectors can keep sending readings through a
long-lived backend connection.

Create this meter before running the example:

```text
name: stream_device_energy_wh
unit: watt-hour
aggregation: sum
dimensions:
  device_type: string
  firmware: string
  gateway: string
  region: string
  signal: string
```

Run:

```sh
cd examples/stream/advance/device-telemetry
OPEN_SPANNER_API_KEY=osp_... go run .
```

Optional settings:

```sh
OPEN_SPANNER_GRPC_ADDR=localhost:18090
OPEN_SPANNER_GRPC_METER=stream_device_energy_wh
```
