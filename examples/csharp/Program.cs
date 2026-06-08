using Microsoft.Kiota.Abstractions.Authentication;
using Microsoft.Kiota.Http.HttpClientLibrary;
using OpenSpanner;
using OpenSpanner.Models;

var baseUrl = Environment.GetEnvironmentVariable("OPEN_SPANNER_BASE_URL") ?? "https://api.example.com";

var authProvider = new AnonymousAuthenticationProvider();
var adapter = new HttpClientRequestAdapter(authProvider)
{
    BaseUrl = baseUrl,
};
var client = new OpenSpannerClient(adapter);

var meterName = $"api_requests_{Guid.NewGuid():N}";
var meter = await client.V1.Meters.PostAsync(new MeterCreateRequest
{
    Name = meterName,
    Description = "API request counter",
    Unit = "request",
    Aggregation = "sum",
    EventRetentionDays = 30,
});

var usage = await client.V1.Usages.PostAsync(new UsageCreateRequest
{
    IdempotencyKey = Guid.NewGuid().ToString(),
    Subject = "org_123",
    Meter = meter?.Name ?? meterName,
    Quantity = 1,
    Timestamp = DateTimeOffset.UtcNow.ToString("O"),
});

Console.WriteLine($"meter={meter?.Id} usage={usage?.Id}");
