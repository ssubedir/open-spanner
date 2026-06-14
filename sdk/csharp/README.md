# Open Spanner C# SDK

Generated C# client for the Open Spanner API.

Record usage for a meter that already exists:

```csharp
using Microsoft.Kiota.Abstractions.Authentication;
using Microsoft.Kiota.Http.HttpClientLibrary;
using OpenSpanner;
using OpenSpanner.Models;

var authProvider = new BaseBearerTokenAuthenticationProvider(new EnvironmentApiKeyProvider());
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

sealed class EnvironmentApiKeyProvider : IAccessTokenProvider
{
    public AllowedHostsValidator AllowedHostsValidator { get; } = new();

    public Task<string> GetAuthorizationTokenAsync(
        Uri uri,
        Dictionary<string, object>? additionalAuthenticationContext = null,
        CancellationToken cancellationToken = default)
    {
        return Task.FromResult(Environment.GetEnvironmentVariable("OPEN_SPANNER_API_KEY") ?? "");
    }
}
```
