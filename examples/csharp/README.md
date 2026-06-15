# C# Basic Example

This example creates a meter, then records usage with the generated C# SDK.

```sh
dotnet run --project OpenSpanner.Example.csproj
```

The example installs the `OpenSpanner` SDK package from NuGet.

Create an API key in the dashboard first, then replace `osp_...` in `Program.cs`.
Set `OPEN_SPANNER_BASE_URL` when your Open Spanner API is not running at
`http://localhost:18081`.
