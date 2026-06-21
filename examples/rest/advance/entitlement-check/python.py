from __future__ import annotations

import os
import time
from datetime import UTC, datetime

from open_spanner_client import AuthenticatedClient
from open_spanner_client.api.plans import check_entitlement
from open_spanner_client.api.usages import create_usage
from open_spanner_client.models.entitlement_check_request import EntitlementCheckRequest
from open_spanner_client.models.usage_create_request import UsageCreateRequest
from open_spanner_client.models.usage_create_request_metadata import UsageCreateRequestMetadata

base_url = os.environ.get("OPEN_SPANNER_BASE_URL", "http://localhost:18081")
api_key = os.environ.get("OPEN_SPANNER_API_KEY", "osp_...")
meter = os.environ.get("OPEN_SPANNER_METER", "api_calls")
subject = os.environ.get("OPEN_SPANNER_SUBJECT", "org_123")
quantity = float(os.environ.get("OPEN_SPANNER_QUANTITY", "1"))

client = AuthenticatedClient(base_url=base_url, token=api_key, raise_on_unexpected_status=True)

entitlement = check_entitlement.sync(
    client=client,
    body=EntitlementCheckRequest(subject=subject, meter=meter, quantity=quantity),
)

print(
    f"{entitlement.subject} on {entitlement.plan_name}: "
    f"allowed={entitlement.allowed} state={entitlement.state} remaining={entitlement.remaining}"
)

if entitlement.allowed:
    metadata = UsageCreateRequestMetadata()
    metadata["source"] = "entitlement-check-example"
    create_usage.sync(
        client=client,
        body=UsageCreateRequest(
            idempotency_key=f"entitlement-check-{subject}-{time.time_ns()}",
            subject=subject,
            meter=meter,
            quantity=quantity,
            timestamp=datetime.now(UTC).replace(microsecond=0).isoformat().replace("+00:00", "Z"),
            metadata=metadata,
        ),
    )

    print("usage accepted")

