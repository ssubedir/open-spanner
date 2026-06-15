using Microsoft.Kiota.Abstractions.Authentication;
using Microsoft.Kiota.Http.HttpClientLibrary;
using OpenSpanner;
using OpenSpanner.Models;

var baseUrl = Environment.GetEnvironmentVariable("OPEN_SPANNER_BASE_URL") ?? "http://localhost:18081";
var apiKey = Environment.GetEnvironmentVariable("OPEN_SPANNER_API_KEY") ?? "osp_...";

var authProvider = new BaseBearerTokenAuthenticationProvider(new ApiKeyProvider(apiKey));
var adapter = new HttpClientRequestAdapter(authProvider)
{
    BaseUrl = baseUrl,
};
var client = new OpenSpannerClient(adapter);

var meterName = $"sdk_csharp_requests_{DateTimeOffset.UtcNow.ToUnixTimeSeconds()}";
var metadataSchema = new MeterCreateRequest_metadata_schema();
metadataSchema.AdditionalData["plan"] = "string";
metadataSchema.AdditionalData["region"] = "string";

var meter = await client.V1.Meters.PostAsync(new MeterCreateRequest
{
    Name = meterName,
    Description = "C# SDK example request counter",
    Unit = "request",
    Aggregation = "sum",
    EventRetentionDays = 30,
    MetadataSchema = metadataSchema,
});

var metadata = new UsageCreateRequest_metadata();
metadata.AdditionalData["plan"] = "pro";
metadata.AdditionalData["region"] = "us-east";

var usage = await client.V1.Usages.PostAsync(new UsageCreateRequest
{
    IdempotencyKey = Guid.NewGuid().ToString(),
    Subject = "org_sdk_csharp",
    Meter = meter?.Name ?? meterName,
    Quantity = 42,
    Timestamp = DateTimeOffset.UtcNow.ToString("O"),
    Metadata = metadata,
});

Console.WriteLine($"created meter: {meter?.Name} ({meter?.Id})");
Console.WriteLine($"recorded usage: {usage?.Id} quantity={usage?.Quantity:F2}");

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
