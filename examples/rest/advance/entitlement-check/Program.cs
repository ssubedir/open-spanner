using Microsoft.Kiota.Abstractions.Authentication;
using Microsoft.Kiota.Http.HttpClientLibrary;
using OpenSpanner;
using OpenSpanner.Models;

var baseUrl = Environment.GetEnvironmentVariable("OPEN_SPANNER_BASE_URL") ?? "http://localhost:18081";
var apiKey = Environment.GetEnvironmentVariable("OPEN_SPANNER_API_KEY") ?? "osp_...";
var meter = Environment.GetEnvironmentVariable("OPEN_SPANNER_METER") ?? "api_calls";
var subject = Environment.GetEnvironmentVariable("OPEN_SPANNER_SUBJECT") ?? "org_123";
var quantity = double.Parse(Environment.GetEnvironmentVariable("OPEN_SPANNER_QUANTITY") ?? "1");

var adapter = new HttpClientRequestAdapter(new BaseBearerTokenAuthenticationProvider(new ApiKeyProvider(apiKey))) { BaseUrl = baseUrl };
var client = new OpenSpannerClient(adapter);

var entitlement = await client.V1.Entitlements.Check.PostAsync(new EntitlementCheckRequest
{
    Subject = subject,
    Meter = meter,
    Quantity = quantity,
});

Console.WriteLine($"{entitlement?.Subject} on {entitlement?.PlanName}: allowed={entitlement?.Allowed} state={entitlement?.State} remaining={entitlement?.Remaining}");

if (entitlement?.Allowed == true)
{
    var metadata = new UsageCreateRequest_metadata();
    metadata.AdditionalData["source"] = "entitlement-check-example";

    await client.V1.Usages.PostAsync(new UsageCreateRequest
    {
        IdempotencyKey = $"entitlement-check-{subject}-{DateTimeOffset.UtcNow.ToUnixTimeMilliseconds()}",
        Subject = subject,
        Meter = meter,
        Quantity = quantity,
        Timestamp = DateTimeOffset.UtcNow.ToString("O"),
        Metadata = metadata,
    });

    Console.WriteLine("usage accepted");
}

sealed class ApiKeyProvider(string apiKey) : IAccessTokenProvider
{
    public AllowedHostsValidator AllowedHostsValidator { get; } = new();

    public Task<string> GetAuthorizationTokenAsync(Uri uri, Dictionary<string, object>? additionalAuthenticationContext = null, CancellationToken cancellationToken = default)
    {
        return Task.FromResult(apiKey);
    }
}

