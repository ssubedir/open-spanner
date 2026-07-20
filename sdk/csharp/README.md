# Open Spanner C# SDK

Generated C# client for the Open Spanner API.

## gRPC streaming

Use the stream client from trusted backend code when you want to send usage through the gRPC ingestion service:

```csharp
using OpenSpanner.Streaming;

var client = new StreamClient(
    "http://localhost:18090",
    "osp_live_...",
    retryPolicy: new RetryPolicy
    {
        MaxAttempts = 3,
        OnRetry = retry => Console.WriteLine($"retry {retry.Attempt} after {retry.Delay}"),
    });
var result = await client.TrackBulkAsync(
    idempotencyKey: Guid.NewGuid().ToString(),
    events:
    [
        new Event
        {
            IdempotencyKey = Guid.NewGuid().ToString(),
            Subject = "org_123",
            Meter = "api_requests",
            Quantity = 1,
            Timestamp = DateTimeOffset.UtcNow,
            Metadata = new Dictionary<string, object?>
            {
                ["endpoint"] = "/checkout",
                ["status"] = 200,
            },
        },
    ]);

Console.WriteLine($"accepted={result.AcceptedCount} failed={result.FailedCount}");
```

Retries are opt-in and apply to unary `TrackAsync` and `TrackBulkAsync` calls. They honor `google.rpc.RetryInfo` and retry only overload, temporary unavailability, and deadline failures. Client streams are not replayed automatically; retry an uncertain stream through `TrackBulkAsync` with the original event idempotency keys.

## REST client

Record usage for a meter that already exists:

```csharp
using Microsoft.Kiota.Abstractions.Authentication;
using Microsoft.Kiota.Http.HttpClientLibrary;
using OpenSpanner;
using OpenSpanner.Models;
using OpenSpanner.Retrying;

var apiKey = "...";
var authProvider = new BaseBearerTokenAuthenticationProvider(new ApiKeyProvider(apiKey));
var adapter = new HttpClientRequestAdapter(authProvider)
{
    BaseUrl = "https://api.example.com",
};
var client = new OpenSpannerClient(adapter);

var request = new UsageCreateRequest
{
    IdempotencyKey = Guid.NewGuid().ToString(),
    Subject = "org_123",
    Meter = "api_requests",
    Quantity = 1,
    Timestamp = DateTimeOffset.UtcNow.ToString("O"),
};
var usage = await UsageRetry.ExecuteAsync(
    cancellationToken => client.V1.Usages.PostAsync(request, cancellationToken: cancellationToken));

Console.WriteLine(usage?.Id);

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
```

`UsageRetry.ExecuteAsync` retries transport failures, `429`, and temporary `5xx` responses, and honors `Retry-After`.
