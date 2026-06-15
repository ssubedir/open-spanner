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
var meterName = $"sdk_csharp_jobs_processed_{runId}";

await client.V1.Meters.PostAsync(new MeterCreateRequest
{
    Name = meterName,
    Description = "Track asynchronous work by queue, result status, and worker region",
    Unit = "job",
    Aggregation = "sum",
    EventRetentionDays = 90,
    Dimensions =
    [
        new() { Name = "queue", DisplayName = "Queue", Description = "Queue or worker pool", Type = "string", Required = true },
        new() { Name = "status", DisplayName = "Status", Description = "Processing result", Type = "string", Required = true },
        new() { Name = "worker_region", DisplayName = "Worker region", Description = "Worker region", Type = "string", Required = false },
    ],
});

var events = new[]
{
    Event("org_acme", 1200, new() { ["queue"] = "exports", ["status"] = "succeeded", ["worker_region"] = "us-east" }),
    Event("org_acme", 28, new() { ["queue"] = "exports", ["status"] = "failed", ["worker_region"] = "us-east" }),
    Event("org_globex", 1145, new() { ["queue"] = "imports", ["status"] = "succeeded", ["worker_region"] = "eu-west" }),
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

Console.WriteLine($"seeded background job meter {meterName} with {events.Length} events");

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
