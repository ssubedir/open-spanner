import { writeFileSync } from "node:fs";

const pyprojectPath = "sdk/python/pyproject.toml";
const readmePath = "sdk/python/README.md";

let pyproject = await import("node:fs").then(({ readFileSync }) => readFileSync(pyprojectPath, "utf8"));
if (!pyproject.includes('license = "MIT"')) {
  pyproject = pyproject.replace(/authors = \[\]\r?\n/, 'authors = []\nlicense = "MIT"\n');
  writeFileSync(pyprojectPath, pyproject);
}

writeFileSync(
  readmePath,
  `# open-spanner

Python client for the Open Spanner API.

Install from PyPI:

\`\`\`sh
pip install open-spanner
\`\`\`

Create a meter, then record usage:

\`\`\`python
from datetime import UTC, datetime
from uuid import uuid4

from open_spanner_client import Client
from open_spanner_client.api.meters import create_meter
from open_spanner_client.api.usages import create_usage
from open_spanner_client.models.meter_create_request import MeterCreateRequest
from open_spanner_client.models.usage_create_request import UsageCreateRequest

client = Client(base_url="https://api.example.com", raise_on_unexpected_status=True)

meter = create_meter.sync(
    client=client,
    body=MeterCreateRequest(
        name="api_requests",
        description="API request counter",
        unit="request",
        aggregation="sum",
        event_retention_days=30,
    ),
)

usage = create_usage.sync(
    client=client,
    body=UsageCreateRequest(
        idempotency_key=str(uuid4()),
        subject="org_123",
        meter=meter.name,
        quantity=1,
        timestamp=datetime.now(UTC).isoformat(),
    ),
)

print(meter.id, usage.id)
\`\`\`
`,
);
