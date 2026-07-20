# open-spanner

Python client for the Open Spanner API.

Install from PyPI:

```sh
pip install open-spanner
```

Record usage for a meter that already exists:

```python
from datetime import UTC, datetime
from uuid import uuid4

from open_spanner_client import AuthenticatedClient
from open_spanner_client.api.usages import create_usage
from open_spanner_client.models.usage_create_request import UsageCreateRequest
from open_spanner_client.retry import retry_usage_write

api_key = "..."

client = AuthenticatedClient(
    base_url="https://api.example.com",
    token=api_key,
    raise_on_unexpected_status=True,
)

request = UsageCreateRequest(
    idempotency_key=str(uuid4()),
    subject="org_123",
    meter="api_requests",
    quantity=1,
    timestamp=datetime.now(UTC).isoformat(),
)
response = retry_usage_write(lambda: create_usage.sync_detailed(client=client, body=request))
usage = response.parsed

print(usage.id)
```

`retry_usage_write` and `retry_usage_write_async` retry transport failures, `429`, and temporary `5xx` responses, and honor `Retry-After`.

Stream usage over gRPC:

```python
from datetime import UTC, datetime

from open_spanner_client.stream import Event, RetryPolicy, StreamClient

client = StreamClient(
    "localhost:18090",
    "osp_...",
    retry_policy=RetryPolicy(
        max_attempts=3,
        on_retry=lambda event: print("retry", event.attempt, event.delay_seconds),
    ),
)
try:
    result = client.track_bulk(
        "batch-1",
        [
            Event(
                idempotency_key="usage-1",
                subject="org_123",
                meter="api_requests",
                quantity=1,
                timestamp=datetime.now(UTC),
                metadata={"endpoint": "/v1/orders", "status": 200},
            )
        ],
    )
finally:
    client.close()

print(result.accepted_count)
```

Retries are opt-in and apply to unary `track` and `track_bulk` calls. They honor `google.rpc.RetryInfo` and retry only overload, temporary unavailability, and deadline failures. Client streams are not replayed automatically; retry an uncertain stream through `track_bulk` with the original event idempotency keys.
