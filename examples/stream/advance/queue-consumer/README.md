# Queue Consumer Stream Example

Streams message-consumption usage from a queue consumer. This is useful when a
worker continuously processes Kafka, NATS, SQS, or Pub/Sub messages and wants to
emit usage by queue, topic, partition, and outcome.

Create this meter before running the example:

```text
name: stream_queue_messages
unit: message
aggregation: sum
dimensions:
  queue: string
  topic: string
  consumer_group: string
  partition: number
  outcome: string
  region: string
```

Run:

```sh
cd examples/stream/advance/queue-consumer
OPEN_SPANNER_API_KEY=osp_... go run .
```

Optional settings:

```sh
OPEN_SPANNER_GRPC_ADDR=localhost:18090
OPEN_SPANNER_GRPC_METER=stream_queue_messages
```
