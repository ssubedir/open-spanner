import { readFileSync, writeFileSync } from "node:fs";

const pyprojectPath = "sdk/python/pyproject.toml";
const readmePath = "sdk/python/README.md";

const normalizeLineEndings = (value) => value.replace(/\r\n?/g, "\n");

let pyproject = normalizeLineEndings(readFileSync(pyprojectPath, "utf8"));
if (!pyproject.includes('license = "MIT"')) {
  pyproject = pyproject.replace(/authors = \[\]\n/, 'authors = []\nlicense = "MIT"\n');
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
import os
from uuid import uuid4

from open_spanner_client import AuthenticatedClient
from open_spanner_client.api.usages import create_usage
from open_spanner_client.models.usage_create_request import UsageCreateRequest

client = AuthenticatedClient(
    base_url="https://api.example.com",
    token=os.environ["OPEN_SPANNER_API_KEY"],
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
`,
);
