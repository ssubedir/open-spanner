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
var meterName = $"sdk_csharp_storage_gb_hours_{runId}";

await client.V1.Meters.PostAsync(new MeterCreateRequest
{
    Name = meterName,
    Description = "Track storage consumption by tier, region, and resource type",
    Unit = "gb_hour",
    Aggregation = "sum",
    EventRetentionDays = 90,
    Dimensions =
    [
        new() { Name = "tier", DisplayName = "Tier", Description = "Storage tier", Type = "string", Required = true },
        new() { Name = "region", DisplayName = "Region", Description = "Storage region", Type = "string", Required = true },
        new() { Name = "resource_type", DisplayName = "Resource type", Description = "Stored resource type", Type = "string", Required = true },
    ],
});

var events = new[]
{
    Event("org_acme", 512.5, new() { ["tier"] = "hot", ["region"] = "us-east", ["resource_type"] = "object" }),
    Event("org_acme", 128, new() { ["tier"] = "archive", ["region"] = "us-east", ["resource_type"] = "backup" }),
    Event("org_globex", 74.25, new() { ["tier"] = "hot", ["region"] = "eu-west", ["resource_type"] = "object" }),
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

Console.WriteLine($"seeded storage usage meter {meterName} with {events.Length} events");

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
