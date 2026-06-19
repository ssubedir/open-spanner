using Microsoft.Kiota.Abstractions.Authentication;
using Microsoft.Kiota.Http.HttpClientLibrary;
using OpenSpanner;
using OpenSpanner.Models;

var baseUrl = Environment.GetEnvironmentVariable("OPEN_SPANNER_BASE_URL") ?? "http://localhost:18081";
var apiKey = Environment.GetEnvironmentVariable("OPEN_SPANNER_API_KEY") ?? "osp_...";
var adapter = new HttpClientRequestAdapter(new BaseBearerTokenAuthenticationProvider(new ApiKeyProvider(apiKey))) { BaseUrl = baseUrl };
var client = new OpenSpannerClient(adapter);

var now = DateTimeOffset.UtcNow;
var runId = now.ToUnixTimeMilliseconds();
var meterName = $"sdk_csharp_active_users_{runId}";

await client.V1.Meters.PostAsync(new MeterCreateRequest
{
    Name = meterName,
    Description = "Track billable active users by plan, workspace type, and region",
    Unit = "user",
    Aggregation = "sum",
    EventRetentionDays = 90,
    Dimensions =
    [
        new() { Name = "plan", DisplayName = "Plan", Description = "Customer plan", Type = "string", Required = true },
        new() { Name = "workspace_type", DisplayName = "Workspace type", Description = "Workspace segment", Type = "string", Required = false },
        new() { Name = "region", DisplayName = "Region", Description = "Primary customer region", Type = "string", Required = false },
    ],
});

var events = new[]
{
    Event("org_acme", 128, new() { ["plan"] = "enterprise", ["workspace_type"] = "production", ["region"] = "us-east" }),
    Event("org_globex", 76, new() { ["plan"] = "business", ["workspace_type"] = "production", ["region"] = "eu-west" }),
    Event("org_initech", 42, new() { ["plan"] = "starter", ["workspace_type"] = "sandbox", ["region"] = "us-west" }),
};

for (var index = 0; index < events.Length; index++)
{
    var item = events[index];
    var metadata = new UsageCreateRequest_metadata();
    foreach (var pair in item.Metadata)
    {
        metadata.AdditionalData[pair.Key] = pair.Value;
    }

    await client.V1.Usages.PostAsync(new UsageCreateRequest
    {
        IdempotencyKey = $"{meterName}-{index}-{runId}",
        Subject = item.Subject,
        Meter = meterName,
        Quantity = item.Quantity,
        Timestamp = now.AddMinutes(index).ToString("O"),
        Metadata = metadata,
    });
}

Console.WriteLine($"seeded active-user meter {meterName} with {events.Length} events");

static UsageEvent Event(string subject, double quantity, Dictionary<string, object> metadata) => new(subject, quantity, metadata);

sealed record UsageEvent(string Subject, double Quantity, Dictionary<string, object> Metadata);

sealed class ApiKeyProvider(string apiKey) : IAccessTokenProvider
{
    public AllowedHostsValidator AllowedHostsValidator { get; } = new();

    public Task<string> GetAuthorizationTokenAsync(Uri uri, Dictionary<string, object>? additionalAuthenticationContext = null, CancellationToken cancellationToken = default)
    {
        return Task.FromResult(apiKey);
    }
}
