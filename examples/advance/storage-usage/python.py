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
meter_name = f"sdk_python_storage_gb_hours_{run_id}"

create_meter.sync(
    client=client,
    body=MeterCreateRequest(
        name=meter_name,
        description="Track storage consumption by tier, region, and resource type",
        unit="gb_hour",
        aggregation="sum",
        event_retention_days=90,
        dimensions=[
            MeterDimensionRequest(name="tier", display_name="Tier", description="Storage tier", type_="string", required=True),
            MeterDimensionRequest(name="region", display_name="Region", description="Storage region", type_="string", required=True),
            MeterDimensionRequest(name="resource_type", display_name="Resource type", description="Stored resource type", type_="string", required=True),
        ],
    ),
)

events = [
    ("org_acme", 512.5, {"tier": "hot", "region": "us-east", "resource_type": "object"}),
    ("org_acme", 128, {"tier": "archive", "region": "us-east", "resource_type": "backup"}),
    ("org_globex", 74.25, {"tier": "hot", "region": "eu-west", "resource_type": "object"}),
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

print(f"seeded storage usage meter {meter_name} with {len(events)} events")
