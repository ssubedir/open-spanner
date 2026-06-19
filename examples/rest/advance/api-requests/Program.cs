using Microsoft.Kiota.Abstractions.Authentication;
using Microsoft.Kiota.Http.HttpClientLibrary;
using OpenSpanner;
using OpenSpanner.Models;

var baseUrl = Environment.GetEnvironmentVariable("OPEN_SPANNER_BASE_URL") ?? "http://localhost:18081";
var apiKey = Environment.GetEnvironmentVariable("OPEN_SPANNER_API_KEY") ?? "osp_...";
var authProvider = new BaseBearerTokenAuthenticationProvider(new ApiKeyProvider(apiKey));
var adapter = new HttpClientRequestAdapter(authProvider) { BaseUrl = baseUrl };
var client = new OpenSpannerClient(adapter);

var now = DateTimeOffset.UtcNow;
var runId = now.ToUnixTimeMilliseconds();
var meterName = $"sdk_csharp_api_requests_{runId}";

await client.V1.Meters.PostAsync(new MeterCreateRequest
{
    Name = meterName,
    Description = "Track request volume by endpoint, method, status, region, and service tier",
    Unit = "request",
    Aggregation = "sum",
    EventRetentionDays = 90,
    Dimensions =
    [
        new() { Name = "endpoint", DisplayName = "Endpoint", Description = "Route or operation", Type = "string", Required = true },
        new() { Name = "method", DisplayName = "Method", Description = "HTTP method", Type = "string", Required = true },
        new() { Name = "status_code", DisplayName = "Status code", Description = "HTTP status code", Type = "number", Required = true },
        new() { Name = "region-name", DisplayName = "Region", Description = "Serving region", Type = "string", Required = false },
        new() { Name = "service.tier", DisplayName = "Service tier", Description = "Backend service tier", Type = "string", Required = true },
    ],
});

var events = new[]
{
    Event("org_acme", 38, new() { ["endpoint"] = "/v1/orders", ["method"] = "POST", ["status_code"] = 201, ["region-name"] = "us-east", ["service"] = new Dictionary<string, object> { ["tier"] = "gold" } }),
    Event("org_acme", 91, new() { ["endpoint"] = "/v1/orders", ["method"] = "GET", ["status_code"] = 200, ["region-name"] = "us-east", ["service"] = new Dictionary<string, object> { ["tier"] = "gold" } }),
    Event("org_globex", 14, new() { ["endpoint"] = "/v1/invoices", ["method"] = "GET", ["status_code"] = 200, ["region-name"] = "eu-west", ["service"] = new Dictionary<string, object> { ["tier"] = "silver" } }),
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

Console.WriteLine($"seeded API request meter {meterName} with {events.Length} events");

static UsageEvent Event(string subject, double quantity, Dictionary<string, object> metadata)
{
    return new UsageEvent(subject, quantity, metadata);
}

sealed record UsageEvent(string Subject, double Quantity, Dictionary<string, object> Metadata);

sealed class ApiKeyProvider(string apiKey) : IAccessTokenProvider
{
    public AllowedHostsValidator AllowedHostsValidator { get; } = new();

    public Task<string> GetAuthorizationTokenAsync(
        Uri uri,
        Dictionary<string, object>? additionalAuthenticationContext = null,
        CancellationToken cancellationToken = default)
    {
        return Task.FromResult(apiKey);
    }
}
