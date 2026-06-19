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
var meterName = $"sdk_csharp_billing_events_backfilled_{runId}";

await client.V1.Meters.PostAsync(new MeterCreateRequest
{
    Name = meterName,
    Description = "Import historical billing events with stable idempotency keys",
    Unit = "event",
    Aggregation = "sum",
    EventRetentionDays = 90,
    Dimensions =
    [
        new() { Name = "source", DisplayName = "Source", Description = "Imported source system", Type = "string", Required = true },
        new() { Name = "event_type", DisplayName = "Event type", Description = "Imported billing event type", Type = "string", Required = true },
        new() { Name = "import_batch", DisplayName = "Import batch", Description = "Backfill batch identifier", Type = "string", Required = true },
    ],
});

var events = new[]
{
    Event("org_acme", 340, -1440, new() { ["source"] = "legacy-billing", ["event_type"] = "api_request", ["import_batch"] = "batch-2026-06" }),
    Event("org_globex", 112, -720, new() { ["source"] = "legacy-billing", ["event_type"] = "storage", ["import_batch"] = "batch-2026-06" }),
    Event("org_initech", 64, -60, new() { ["source"] = "csv-import", ["event_type"] = "feature_use", ["import_batch"] = "batch-2026-06" }),
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
        IdempotencyKey = $"{meterName}-{item.Metadata["import_batch"]}-{item.Subject}-{index}",
        Subject = item.Subject,
        Meter = meterName,
        Quantity = item.Quantity,
        Timestamp = now.AddMinutes(item.OffsetMinutes).ToString("O"),
        Metadata = metadata,
    });
}

Console.WriteLine($"seeded historical backfill meter {meterName} with {events.Length} events");

static UsageEvent Event(string subject, double quantity, int offsetMinutes, Dictionary<string, object> metadata) => new(subject, quantity, offsetMinutes, metadata);

sealed record UsageEvent(string Subject, double Quantity, int OffsetMinutes, Dictionary<string, object> Metadata);

sealed class ApiKeyProvider(string apiKey) : IAccessTokenProvider
{
    public AllowedHostsValidator AllowedHostsValidator { get; } = new();

    public Task<string> GetAuthorizationTokenAsync(Uri uri, Dictionary<string, object>? additionalAuthenticationContext = null, CancellationToken cancellationToken = default)
    {
        return Task.FromResult(apiKey);
    }
}
