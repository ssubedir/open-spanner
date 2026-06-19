# C# Basic Example

This example creates a meter, then records usage with the C# SDK from this checkout.

```sh
dotnet run --project OpenSpanner.Example.csproj
```

Create an API key in the dashboard first, then replace `osp_...` in `Program.cs`.
Set `OPEN_SPANNER_BASE_URL` when your Open Spanner API is not running at
`http://localhost:18081`.

Advanced use-case examples live under `../../advance`.
