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
var meterName = $"sdk_csharp_feature_uses_{runId}";

await client.V1.Meters.PostAsync(new MeterCreateRequest
{
    Name = meterName,
    Description = "Track usage of premium features and add-ons by customer plan",
    Unit = "use",
    Aggregation = "sum",
    EventRetentionDays = 90,
    Dimensions =
    [
        new() { Name = "feature", DisplayName = "Feature", Description = "Product feature or add-on", Type = "string", Required = true },
        new() { Name = "plan", DisplayName = "Plan", Description = "Customer plan", Type = "string", Required = true },
        new() { Name = "source", DisplayName = "Source", Description = "UI, API, automation, or integration", Type = "string", Required = false },
    ],
});

var events = new[]
{
    Event("org_acme", 48, new() { ["feature"] = "audit_exports", ["plan"] = "enterprise", ["source"] = "ui" }),
    Event("org_acme", 19, new() { ["feature"] = "custom_reports", ["plan"] = "enterprise", ["source"] = "api" }),
    Event("org_globex", 8, new() { ["feature"] = "custom_reports", ["plan"] = "business", ["source"] = "automation" }),
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

Console.WriteLine($"seeded feature usage meter {meterName} with {events.Length} events");

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
