using System.Diagnostics;
using System.Net;
using System.Net.Http.Json;
using System.Net.Sockets;
using System.Text;
using System.Text.Json;
using OpenSpanner.Streaming;

var httpAddr = FreeTcpAddr();
var grpcAddr = FreeTcpAddr();

await using var service = await OpenSpannerService.StartAsync(httpAddr, grpcAddr);

var suffix = DateTimeOffset.UtcNow.ToUnixTimeMilliseconds().ToString();
var apiKey = await CreateApiKeyAsync(service.BaseUrl, suffix);
var meterName = "sdk_csharp_stream_requests_" + suffix;
await CreateMeterAsync(service.BaseUrl, apiKey, meterName);

using var client = new StreamClient(grpcAddr, apiKey);
var now = DateTimeOffset.UtcNow;

var bulk = await client.TrackBulkAsync(
    "sdk-csharp-stream-bulk-" + suffix,
    new[]
    {
        UsageEvent("sdk-csharp-stream-bulk-" + suffix + "-1", "org_sdk_csharp_stream_" + suffix, meterName, 2, now, new Dictionary<string, object?> { ["endpoint"] = "/orders", ["status"] = 200 }),
        UsageEvent("sdk-csharp-stream-bulk-" + suffix + "-2", "org_sdk_csharp_stream_" + suffix, meterName, 3, now.AddSeconds(1), new Dictionary<string, object?> { ["endpoint"] = "/users", ["status"] = 201 }),
    });

AssertEqual(2, bulk.AcceptedCount, "bulk accepted");
AssertEqual(0, bulk.DuplicateCount, "bulk duplicates");
AssertEqual(0, bulk.FailedCount, "bulk failed");

var usageStream = client.Stream("sdk-csharp-stream-" + suffix);
await usageStream.TrackAsync(UsageEvent("sdk-csharp-stream-" + suffix + "-1", "org_sdk_csharp_stream_" + suffix, meterName, 7, now.AddSeconds(2), new Dictionary<string, object?> { ["endpoint"] = "/checkout", ["status"] = 200 }));
var streamed = await usageStream.CloseAsync();

AssertEqual(1, streamed.AcceptedCount, "stream accepted");
AssertEqual(0, streamed.DuplicateCount, "stream duplicates");
AssertEqual(0, streamed.FailedCount, "stream failed");

var events = await ListUsageEventsAsync(service.BaseUrl, apiKey, meterName);
AssertEqual(3, events.Items.Length, "usage event count");

static Event UsageEvent(string idempotencyKey, string subject, string meter, double quantity, DateTimeOffset timestamp, IReadOnlyDictionary<string, object?> metadata)
{
    return new Event
    {
        IdempotencyKey = idempotencyKey,
        Subject = subject,
        Meter = meter,
        Quantity = quantity,
        Timestamp = timestamp,
        Metadata = metadata,
    };
}

static async Task<string> CreateApiKeyAsync(string baseUrl, string suffix)
{
    var handler = new HttpClientHandler
    {
        CookieContainer = new CookieContainer(),
    };
    using var client = new HttpClient(handler)
    {
        Timeout = TimeSpan.FromSeconds(5),
    };

    var email = "sdk-csharp-stream+" + suffix + "@example.com";
    var password = "strong-password";

    await PostJsonAsync<object>(client, baseUrl + "/v1/auth/users", new Dictionary<string, object?>
    {
        ["email"] = email,
        ["password"] = password,
    }, HttpStatusCode.Created);

    await PostJsonAsync<object>(client, baseUrl + "/v1/auth/sessions", new Dictionary<string, object?>
    {
        ["email"] = email,
        ["password"] = password,
    }, HttpStatusCode.Created);

    var apiKey = await PostJsonAsync<ApiKeyResponse>(client, baseUrl + "/v1/auth/api-keys", new Dictionary<string, object?>
    {
        ["name"] = "sdk csharp stream test " + suffix,
    }, HttpStatusCode.Created);

    if (string.IsNullOrWhiteSpace(apiKey.Key))
    {
        throw new InvalidOperationException("API key response did not include a key.");
    }
    return apiKey.Key;
}

static async Task CreateMeterAsync(string baseUrl, string apiKey, string meterName)
{
    using var client = new HttpClient
    {
        Timeout = TimeSpan.FromSeconds(5),
    };
    client.DefaultRequestHeaders.Authorization = new System.Net.Http.Headers.AuthenticationHeaderValue("Bearer", apiKey);

    await PostJsonAsync<object>(client, baseUrl + "/v1/meters", new Dictionary<string, object?>
    {
        ["name"] = meterName,
        ["description"] = "C# SDK stream integration requests",
        ["unit"] = "request",
        ["aggregation"] = "sum",
        ["event_retention_days"] = 30,
        ["dimensions"] = new[]
        {
            new Dictionary<string, object?> { ["name"] = "endpoint", ["type"] = "string", ["required"] = true },
            new Dictionary<string, object?> { ["name"] = "status", ["type"] = "number", ["required"] = true },
        },
    }, HttpStatusCode.Created);
}

static async Task<UsageEventList> ListUsageEventsAsync(string baseUrl, string apiKey, string meterName)
{
    using var client = new HttpClient
    {
        Timeout = TimeSpan.FromSeconds(5),
    };
    client.DefaultRequestHeaders.Authorization = new System.Net.Http.Headers.AuthenticationHeaderValue("Bearer", apiKey);

    using var response = await client.GetAsync(baseUrl + "/v1/usageevents?meter=" + Uri.EscapeDataString(meterName) + "&limit=10");
    var body = await response.Content.ReadAsStringAsync();
    if (response.StatusCode != HttpStatusCode.OK)
    {
        throw new InvalidOperationException("List usage events failed: " + response.StatusCode + " " + body);
    }
    return JsonSerializer.Deserialize<UsageEventList>(body, JsonSupport.Options) ?? throw new InvalidOperationException("Usage event response was empty.");
}

static async Task<T> PostJsonAsync<T>(HttpClient client, string url, object body, HttpStatusCode wantStatus)
{
    var content = new StringContent(JsonSerializer.Serialize(body, JsonSupport.Options), Encoding.UTF8, "application/json");
    using var response = await client.PostAsync(url, content);
    var responseBody = await response.Content.ReadAsStringAsync();
    if (response.StatusCode != wantStatus)
    {
        throw new InvalidOperationException("POST " + url + " failed: " + response.StatusCode + " " + responseBody);
    }
    if (typeof(T) == typeof(object) || string.IsNullOrWhiteSpace(responseBody))
    {
        return default!;
    }
    return JsonSerializer.Deserialize<T>(responseBody, JsonSupport.Options) ?? throw new InvalidOperationException("Response body was empty.");
}

static string FreeTcpAddr()
{
    var listener = new TcpListener(IPAddress.Loopback, 0);
    listener.Start();
    var endpoint = (IPEndPoint)listener.LocalEndpoint;
    listener.Stop();
    return endpoint.Address + ":" + endpoint.Port;
}

static void AssertEqual<T>(T expected, T actual, string name)
{
    if (!EqualityComparer<T>.Default.Equals(expected, actual))
    {
        throw new InvalidOperationException(name + " = " + actual + ", want " + expected);
    }
}

sealed record ApiKeyResponse(string Key);

sealed record UsageEventList(UsageEventItem[] Items);

sealed record UsageEventItem(string Meter, string Subject, double Quantity, Dictionary<string, object?> Metadata);

static class JsonSupport
{
    public static readonly JsonSerializerOptions Options = new(JsonSerializerDefaults.Web)
    {
        PropertyNamingPolicy = JsonNamingPolicy.SnakeCaseLower,
    };
}

sealed class OpenSpannerService : IAsyncDisposable
{
    private readonly Process process;
    private readonly string tempDir;

    private OpenSpannerService(Process process, string tempDir, string baseUrl)
    {
        this.process = process;
        this.tempDir = tempDir;
        BaseUrl = baseUrl;
    }

    public string BaseUrl { get; }

    public static async Task<OpenSpannerService> StartAsync(string httpAddr, string grpcAddr)
    {
        var repoRoot = Path.GetFullPath(Path.Combine(AppContext.BaseDirectory, "..", "..", "..", "..", "..", ".."));
        var tempDir = Directory.CreateTempSubdirectory("open-spanner-sdk-csharp-").FullName;
        var binaryPath = Path.Combine(tempDir, OperatingSystem.IsWindows() ? "open-spanner-sdk-test.exe" : "open-spanner-sdk-test");

        var buildInfo = new ProcessStartInfo
        {
            FileName = "go",
            ArgumentList = { "build", "-o", binaryPath, "./cmd/api" },
            WorkingDirectory = repoRoot,
            RedirectStandardError = true,
            RedirectStandardOutput = true,
        };
        buildInfo.Environment["GOCACHE"] = Path.Combine(repoRoot, ".tmp", "go-build");

        var build = Process.Start(buildInfo) ?? throw new InvalidOperationException("Could not start Go build.");
        await build.WaitForExitAsync();
        if (build.ExitCode != 0)
        {
            throw new InvalidOperationException("Go build failed:\n" + await build.StandardOutput.ReadToEndAsync() + await build.StandardError.ReadToEndAsync());
        }

        var start = new ProcessStartInfo
        {
            FileName = binaryPath,
            WorkingDirectory = repoRoot,
            RedirectStandardError = true,
            RedirectStandardOutput = true,
        };
        start.Environment["OPEN_SPANNER_HTTP_ADDR"] = httpAddr;
        start.Environment["OPEN_SPANNER_GRPC_ADDR"] = grpcAddr;
        start.Environment["OPEN_SPANNER_DB_DRIVER"] = "sqlite";
        start.Environment["OPEN_SPANNER_SQLITE_PATH"] = Path.Combine(tempDir, "open-spanner.db");
        start.Environment["OPEN_SPANNER_EXPORT_STORAGE_PATH"] = Path.Combine(tempDir, "exports");

        var process = Process.Start(start) ?? throw new InvalidOperationException("Could not start Open Spanner.");
        var service = new OpenSpannerService(process, tempDir, "http://" + httpAddr);
        try
        {
            await service.WaitForReadyAsync();
        }
        catch
        {
            await service.DisposeAsync();
            throw;
        }
        return service;
    }

    public async ValueTask DisposeAsync()
    {
        if (!process.HasExited)
        {
            process.Kill(entireProcessTree: true);
            await process.WaitForExitAsync();
        }
        process.Dispose();
        Directory.Delete(tempDir, recursive: true);
    }

    private async Task WaitForReadyAsync()
    {
        using var client = new HttpClient
        {
            Timeout = TimeSpan.FromSeconds(1),
        };
        var deadline = DateTimeOffset.UtcNow.AddSeconds(20);
        while (DateTimeOffset.UtcNow < deadline)
        {
            if (process.HasExited)
            {
                throw new InvalidOperationException("Open Spanner exited before ready.");
            }
            try
            {
                using var response = await client.GetAsync(BaseUrl + "/ready");
                if (response.StatusCode == HttpStatusCode.NoContent)
                {
                    return;
                }
            }
            catch (HttpRequestException)
            {
            }
            catch (TaskCanceledException)
            {
            }
            await Task.Delay(100);
        }
        throw new TimeoutException("Open Spanner did not become ready.");
    }
}
