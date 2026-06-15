from __future__ import annotations

import os
import time
from datetime import UTC, datetime, timedelta

from open_spanner_client import AuthenticatedClient
from open_spanner_client.api.meters import create_meter
from open_spanner_client.api.usages import create_usage, search_usage_buckets
from open_spanner_client.models.error_response import ErrorResponse
from open_spanner_client.models.internal_metering_adapters_http_usage_search_request import (
    InternalMeteringAdaptersHttpUsageSearchRequest,
)
from open_spanner_client.models.meter_create_request import MeterCreateRequest
from open_spanner_client.models.meter_create_request_metadata_schema import MeterCreateRequestMetadataSchema
from open_spanner_client.models.usage_create_request import UsageCreateRequest
from open_spanner_client.models.usage_create_request_metadata import UsageCreateRequestMetadata


def main() -> None:
    base_url = os.environ.get("OPEN_SPANNER_BASE_URL", "http://localhost:18081")
    api_key = os.environ.get("OPEN_SPANNER_API_KEY", "osp_...")
    client = AuthenticatedClient(
        base_url=base_url,
        token=api_key,
        raise_on_unexpected_status=True,
    )

    now = datetime.now(UTC).replace(microsecond=0)
    meter_name = f"sdk_python_requests_{int(time.time())}"
    subject = "org_sdk_python"

    metadata_schema = MeterCreateRequestMetadataSchema()
    metadata_schema["plan"] = "string"
    metadata_schema["region"] = "string"

    meter = create_meter.sync(
        client=client,
        body=MeterCreateRequest(
            name=meter_name,
            description="Python SDK example request counter",
            unit="request",
            aggregation="sum",
            event_retention_days=30,
            metadata_schema=metadata_schema,
        ),
    )

    metadata = UsageCreateRequestMetadata()
    metadata["plan"] = "pro"
    metadata["region"] = "us-east"

    usage = create_usage.sync(
        client=client,
        body=UsageCreateRequest(
            idempotency_key=f"{meter_name}-{time.time_ns()}",
            subject=subject,
            meter=meter_name,
            quantity=42,
            timestamp=now.isoformat().replace("+00:00", "Z"),
            metadata=metadata,
        ),
    )

    buckets = search_usage_buckets.sync(
        client=client,
        body=InternalMeteringAdaptersHttpUsageSearchRequest(
            subject=subject,
            meter=meter_name,
            from_=(now - timedelta(hours=1)).isoformat().replace("+00:00", "Z"),
            to=(now + timedelta(hours=1)).isoformat().replace("+00:00", "Z"),
            bucket_size="hour",
            limit=10,
        ),
    )
    if buckets is None:
        buckets = []
    if isinstance(buckets, ErrorResponse):
        raise RuntimeError(f"usage search failed: {buckets.to_dict()}")

    print(f"created meter: {meter.name} ({meter.id})")
    print(f"recorded usage: {usage.id} quantity={usage.quantity:.2f}")
    print(f"usage buckets: {len(buckets)}")
    for bucket in buckets:
        print(f"- {bucket.bucket_start} {bucket.meter} {bucket.quantity:.2f} {bucket.unit}")

if __name__ == "__main__":
    main()
