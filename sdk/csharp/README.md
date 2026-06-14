# Open Spanner C# SDK

Generated C# client for the Open Spanner API.

Record usage for a meter that already exists:

```csharp
using Microsoft.Kiota.Abstractions.Authentication;
using Microsoft.Kiota.Http.HttpClientLibrary;
using OpenSpanner;
using OpenSpanner.Models;

var apiKey = "...";
var authProvider = new BaseBearerTokenAuthenticationProvider(new ApiKeyProvider(apiKey));
var adapter = new HttpClientRequestAdapter(authProvider)
{
    BaseUrl = "https://api.example.com",
};
var client = new OpenSpannerClient(adapter);

var usage = await client.V1.Usages.PostAsync(new UsageCreateRequest
{
    IdempotencyKey = Guid.NewGuid().ToString(),
    Subject = "org_123",
    Meter = "api_requests",
    Quantity = 1,
    Timestamp = DateTimeOffset.UtcNow.ToString("O"),
});

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
