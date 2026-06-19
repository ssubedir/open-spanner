from __future__ import annotations

import os
import time
from datetime import UTC, datetime, timedelta

from open_spanner_client import AuthenticatedClient
from open_spanner_client.api.meters import create_meter
from open_spanner_client.api.usages import create_usage
from open_spanner_client.models.meter_create_request import MeterCreateRequest
from open_spanner_client.models.meter_dimension_request import MeterDimensionRequest
from open_spanner_client.models.usage_create_request import UsageCreateRequest
from open_spanner_client.models.usage_create_request_metadata import UsageCreateRequestMetadata

base_url = os.environ.get("OPEN_SPANNER_BASE_URL", "http://localhost:18081")
api_key = os.environ.get("OPEN_SPANNER_API_KEY", "osp_...")
client = AuthenticatedClient(base_url=base_url, token=api_key, raise_on_unexpected_status=True)

now = datetime.now(UTC).replace(microsecond=0)
run_id = time.time_ns() // 1_000_000
meter_name = f"sdk_python_tokens_used_{run_id}"

create_meter.sync(
    client=client,
    body=MeterCreateRequest(
        name=meter_name,
        description="Track model token consumption by provider, model, operation, and cache path",
        unit="token",
        aggregation="sum",
        event_retention_days=90,
        dimensions=[
            MeterDimensionRequest(name="model", display_name="Model", description="Model identifier", type_="string", required=True),
            MeterDimensionRequest(name="provider", display_name="Provider", description="AI provider", type_="string", required=True),
            MeterDimensionRequest(name="operation", display_name="Operation", description="Completion, embedding, or rerank", type_="string", required=True),
            MeterDimensionRequest(name="cached", display_name="Cached", description="Whether cached context was used", type_="boolean", required=False),
        ],
    ),
)

events = [
    ("org_acme", 24800, {"model": "gpt-4.1", "provider": "openai", "operation": "completion", "cached": False}),
    ("org_acme", 13200, {"model": "text-embedding-3-large", "provider": "openai", "operation": "embedding", "cached": True}),
    ("org_globex", 4100, {"model": "claude-3-5-sonnet", "provider": "anthropic", "operation": "completion", "cached": False}),
]

for index, (subject, quantity, values) in enumerate(events):
    metadata = UsageCreateRequestMetadata()
    metadata.update(values)
    create_usage.sync(
        client=client,
        body=UsageCreateRequest(
            idempotency_key=f"{meter_name}-{index}-{run_id}",
            subject=subject,
            meter=meter_name,
            quantity=quantity,
            timestamp=(now + timedelta(minutes=index)).isoformat().replace("+00:00", "Z"),
            metadata=metadata,
        ),
    )

print(f"seeded AI token meter {meter_name} with {len(events)} events")
