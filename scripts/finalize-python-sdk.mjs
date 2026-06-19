import { readFileSync, writeFileSync } from "node:fs";

const pyprojectPath = "sdk/python/pyproject.toml";
const readmePath = "sdk/python/README.md";

const normalizeLineEndings = (value) => value.replace(/\r\n?/g, "\n");

let pyproject = normalizeLineEndings(readFileSync(pyprojectPath, "utf8"));
if (!pyproject.includes('license = "MIT"')) {
  pyproject = pyproject.replace(/authors = \[\]\n/, 'authors = []\nlicense = "MIT"\n');
}
if (!pyproject.includes('"grpcio>=1.76.0,<2.0.0"')) {
  pyproject = pyproject.replace(/    "attrs>=22\.2\.0",\n/, '    "attrs>=22.2.0",\n    "grpcio>=1.76.0,<2.0.0",\n    "protobuf>=6.33.1,<7.0.0",\n');
}
writeFileSync(pyprojectPath, pyproject);

writeFileSync(
  readmePath,
  `# open-spanner

Python client for the Open Spanner API.

Install from PyPI:

\`\`\`sh
pip install open-spanner
\`\`\`

Record usage for a meter that already exists:

\`\`\`python
from datetime import UTC, datetime
from uuid import uuid4

from open_spanner_client import AuthenticatedClient
from open_spanner_client.api.usages import create_usage
from open_spanner_client.models.usage_create_request import UsageCreateRequest

api_key = "..."

client = AuthenticatedClient(
    base_url="https://api.example.com",
    token=api_key,
    raise_on_unexpected_status=True,
)

usage = create_usage.sync(
    client=client,
    body=UsageCreateRequest(
        idempotency_key=str(uuid4()),
        subject="org_123",
        meter="api_requests",
        quantity=1,
        timestamp=datetime.now(UTC).isoformat(),
    ),
)

print(usage.id)
\`\`\`

Stream usage over gRPC:

\`\`\`python
from datetime import UTC, datetime

from open_spanner_client.stream import Event, StreamClient

client = StreamClient("localhost:18090", "osp_...")
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
\`\`\`
`,
);
