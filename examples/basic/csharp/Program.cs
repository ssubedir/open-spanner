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

var meter = await client.V1.Meters.PostAsync(new MeterCreateRequest
{
    Name = meterName,
    Description = "C# SDK example request counter",
    Unit = "request",
    Aggregation = "sum",
    EventRetentionDays = 30,
    Dimensions =
    [
        new()
        {
            Name = "endpoint",
            DisplayName = "Endpoint",
            Description = "API route that handled the request",
            Type = "string",
            Required = true,
        },
        new()
        {
            Name = "status",
            DisplayName = "HTTP status",
            Description = "Response status code",
            Type = "number",
            Required = true,
        },
        new()
        {
            Name = "region",
            DisplayName = "Region",
            Description = "Serving region",
            Type = "string",
            Required = false,
        },
    ],
});

var metadata = new UsageCreateRequest_metadata();
metadata.AdditionalData["endpoint"] = "/v1/orders";
metadata.AdditionalData["status"] = 200;
metadata.AdditionalData["region"] = "us-east";
metadata.AdditionalData["trace_id"] = "trace-csharp-example";

var usage = await client.V1.Usages.PostAsync(new UsageCreateRequest
{
    IdempotencyKey = Guid.NewGuid().ToString(),
    Subject = "org_sdk_csharp",
    Meter = meter?.Name ?? meterName,
    Quantity = 42,
    Timestamp = DateTimeOffset.UtcNow.ToString("O"),
    Metadata = metadata,
});

var invalidMetadata = new UsageCreateRequest_metadata();
invalidMetadata.AdditionalData["endpoint"] = "/v1/orders";
invalidMetadata.AdditionalData["status"] = "200";

var validationMessage = "";
try
{
    await client.V1.Usages.PostAsync(new UsageCreateRequest
    {
        IdempotencyKey = Guid.NewGuid().ToString(),
        Subject = "org_sdk_csharp",
        Meter = meter?.Name ?? meterName,
        Quantity = 1,
        Timestamp = DateTimeOffset.UtcNow.ToString("O"),
        Metadata = invalidMetadata,
    });
    throw new InvalidOperationException("expected dimension validation error");
}
catch (ErrorResponse error)
{
    validationMessage = error.Error?.Message ?? error.Message;
}

Console.WriteLine($"created meter: {meter?.Name} ({meter?.Id})");
Console.WriteLine($"recorded usage: {usage?.Id} quantity={usage?.Quantity:F2}");
Console.WriteLine($"dimension validation rejected invalid usage: {validationMessage}");

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
