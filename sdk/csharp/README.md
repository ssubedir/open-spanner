# Open Spanner C# SDK

Generated C# client for the Open Spanner API.

Create a meter, then record usage:

```csharp
using Microsoft.Kiota.Abstractions.Authentication;
using Microsoft.Kiota.Http.HttpClientLibrary;
using OpenSpanner;
using OpenSpanner.Models;

var authProvider = new AnonymousAuthenticationProvider();
var adapter = new HttpClientRequestAdapter(authProvider)
{
    BaseUrl = "https://api.example.com",
};
var client = new OpenSpannerClient(adapter);

var meter = await client.V1.Meters.PostAsync(new MeterCreateRequest
{
    Name = "api_requests",
    Description = "API request counter",
    Unit = "request",
    Aggregation = "sum",
    EventRetentionDays = 30,
});

var usage = await client.V1.Usages.PostAsync(new UsageCreateRequest
{
    IdempotencyKey = Guid.NewGuid().ToString(),
    Subject = "org_123",
    Meter = meter?.Name,
    Quantity = 1,
    Timestamp = DateTimeOffset.UtcNow.ToString("O"),
});

Console.WriteLine($"{meter?.Id} {usage?.Id}");
```
