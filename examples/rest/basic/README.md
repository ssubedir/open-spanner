# Basic SDK Examples

These examples show the smallest useful SDK flow: create one meter, write usage,
and see dimension validation reject invalid metadata.

| SDK | Command |
| --- | --- |
| TypeScript | `cd examples/rest/basic/typescript && npm install && npm run start` |
| Python | `cd examples/rest/basic/python && uv run python basic.py` |
| C# | `cd examples/rest/basic/csharp && dotnet run --project OpenSpanner.Example.csproj` |
| Go | `cd examples/rest/basic/go && go run main.go` |

Create an API key in the dashboard before running an example. Advanced
use-case examples live under `../advance`. gRPC streaming examples live under
`../../stream`.
