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
meter_name = f"sdk_python_billing_events_backfilled_{run_id}"

create_meter.sync(
    client=client,
    body=MeterCreateRequest(
        name=meter_name,
        description="Import historical billing events with stable idempotency keys",
        unit="event",
        aggregation="sum",
        event_retention_days=90,
        dimensions=[
            MeterDimensionRequest(name="source", display_name="Source", description="Imported source system", type_="string", required=True),
            MeterDimensionRequest(name="event_type", display_name="Event type", description="Imported billing event type", type_="string", required=True),
            MeterDimensionRequest(name="import_batch", display_name="Import batch", description="Backfill batch identifier", type_="string", required=True),
        ],
    ),
)

events = [
    ("org_acme", 340, -1440, {"source": "legacy-billing", "event_type": "api_request", "import_batch": "batch-2026-06"}),
    ("org_globex", 112, -720, {"source": "legacy-billing", "event_type": "storage", "import_batch": "batch-2026-06"}),
    ("org_initech", 64, -60, {"source": "csv-import", "event_type": "feature_use", "import_batch": "batch-2026-06"}),
]

for index, (subject, quantity, offset_minutes, values) in enumerate(events):
    metadata = UsageCreateRequestMetadata()
    metadata.update(values)
    create_usage.sync(
        client=client,
        body=UsageCreateRequest(
            idempotency_key=f"{meter_name}-{values['import_batch']}-{subject}-{index}",
            subject=subject,
            meter=meter_name,
            quantity=quantity,
            timestamp=(now + timedelta(minutes=offset_minutes)).isoformat().replace("+00:00", "Z"),
            metadata=metadata,
        ),
    )

print(f"seeded historical backfill meter {meter_name} with {len(events)} events")
