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
var meterName = $"sdk_csharp_tokens_used_{runId}";

await client.V1.Meters.PostAsync(new MeterCreateRequest
{
    Name = meterName,
    Description = "Track model token consumption by provider, model, operation, and cache path",
    Unit = "token",
    Aggregation = "sum",
    EventRetentionDays = 90,
    Dimensions =
    [
        new() { Name = "model", DisplayName = "Model", Description = "Model identifier", Type = "string", Required = true },
        new() { Name = "provider", DisplayName = "Provider", Description = "AI provider", Type = "string", Required = true },
        new() { Name = "operation", DisplayName = "Operation", Description = "Completion, embedding, or rerank", Type = "string", Required = true },
        new() { Name = "cached", DisplayName = "Cached", Description = "Whether cached context was used", Type = "boolean", Required = false },
    ],
});

var events = new[]
{
    Event("org_acme", 24800, new() { ["model"] = "gpt-4.1", ["provider"] = "openai", ["operation"] = "completion", ["cached"] = false }),
    Event("org_acme", 13200, new() { ["model"] = "text-embedding-3-large", ["provider"] = "openai", ["operation"] = "embedding", ["cached"] = true }),
    Event("org_globex", 4100, new() { ["model"] = "claude-3-5-sonnet", ["provider"] = "anthropic", ["operation"] = "completion", ["cached"] = false }),
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

Console.WriteLine($"seeded AI token meter {meterName} with {events.Length} events");

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
