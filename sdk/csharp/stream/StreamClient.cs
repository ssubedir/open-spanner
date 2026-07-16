using Google.Protobuf;
using Google.Protobuf.WellKnownTypes;
using Grpc.Core;
using Grpc.Net.Client;
using OpenSpanner.Grpc.V1;

namespace OpenSpanner.Streaming;

public sealed record Event
{
    public string IdempotencyKey { get; init; } = "";
    public required string Subject { get; init; }
    public required string Meter { get; init; }
    public double Quantity { get; init; }
    public DateTimeOffset Timestamp { get; init; } = DateTimeOffset.UtcNow;
    public IReadOnlyDictionary<string, object?> Metadata { get; init; } = new Dictionary<string, object?>();
}

public sealed record RecordedEvent(
    string Id,
    string IdempotencyKey,
    string Subject,
    string Meter,
    double Quantity,
    DateTimeOffset? Timestamp,
    DateTimeOffset? ReceivedAt,
    IReadOnlyDictionary<string, object?> Metadata);

public sealed record Failure(int Index, string Code, string Message);

public sealed record BulkResult(
    int AcceptedCount,
    int DuplicateCount,
    int FailedCount,
    IReadOnlyList<RecordedEvent> Accepted,
    IReadOnlyList<RecordedEvent> Duplicates,
    IReadOnlyList<Failure> Failed);

public sealed record RetryEvent(int Attempt, TimeSpan Delay, RpcException Error);

public sealed record RetryPolicy
{
    /// <summary>Total attempts including the initial request.</summary>
    public int MaxAttempts { get; init; } = 3;
    public TimeSpan InitialBackoff { get; init; } = TimeSpan.FromMilliseconds(100);
    public TimeSpan MaxBackoff { get; init; } = TimeSpan.FromSeconds(5);
    public double Jitter { get; init; } = 0.2;
    public Action<RetryEvent>? OnRetry { get; init; }
}

public sealed class StreamClient : IDisposable
{
    private readonly GrpcChannel channel;
    private readonly UsageService.UsageServiceClient client;
    private readonly string apiKey;
    private readonly RetryPolicy? retryPolicy;

    public StreamClient(string address, string apiKey, GrpcChannelOptions? options = null, RetryPolicy? retryPolicy = null)
    {
        address = address.Trim();
        if (string.IsNullOrWhiteSpace(address))
        {
            throw new ArgumentException("gRPC address is required.", nameof(address));
        }

        apiKey = apiKey.Trim();
        if (string.IsNullOrWhiteSpace(apiKey))
        {
            throw new ArgumentException("API key is required.", nameof(apiKey));
        }

        this.apiKey = apiKey;
        this.retryPolicy = retryPolicy is null ? null : Normalize(retryPolicy);
        channel = GrpcChannel.ForAddress(NormalizeAddress(address), options ?? new GrpcChannelOptions());
        client = new UsageService.UsageServiceClient(channel);
    }

    public async Task<RecordedEvent> TrackAsync(Event usageEvent, CancellationToken cancellationToken = default)
    {
        var request = new CreateUsageRequest { Event = EventInput(usageEvent) };
        var response = await RetryUnaryAsync(
            () => client.CreateUsageAsync(request, headers: MetadataHeaders(), cancellationToken: cancellationToken).ResponseAsync,
            cancellationToken);
        return Recorded(response.Event);
    }

    public async Task<BulkResult> TrackBulkAsync(string idempotencyKey, IEnumerable<Event> events, CancellationToken cancellationToken = default)
    {
        var request = new CreateUsageBulkRequest
        {
            IdempotencyKey = idempotencyKey,
        };
        request.Events.AddRange(events.Select(EventInput));

        var response = await RetryUnaryAsync(
            () => client.CreateUsageBulkAsync(request, headers: MetadataHeaders(), cancellationToken: cancellationToken).ResponseAsync,
            cancellationToken);
        return Bulk(response);
    }

    public UsageStream Stream(string idempotencyKey, CancellationToken cancellationToken = default)
    {
        var headers = MetadataHeaders(("idempotency-key", idempotencyKey));
        return new UsageStream(client.StreamUsage(headers: headers, cancellationToken: cancellationToken));
    }

    public void Dispose()
    {
        channel.Dispose();
    }

    private Metadata MetadataHeaders(params (string Key, string Value)[] extra)
    {
        var metadata = new Metadata
        {
            { "authorization", $"Bearer {apiKey}" },
        };
        foreach (var (key, value) in extra)
        {
            metadata.Add(key, value);
        }
        return metadata;
    }

    private async Task<T> RetryUnaryAsync<T>(Func<Task<T>> call, CancellationToken cancellationToken)
    {
        if (retryPolicy is null)
        {
            return await call();
        }

        for (var attempt = 1; ; attempt++)
        {
            try
            {
                return await call();
            }
            catch (RpcException error) when (attempt < retryPolicy.MaxAttempts && Retryable(error))
            {
                var delay = RetryDelay(error, retryPolicy, attempt);
                retryPolicy.OnRetry?.Invoke(new RetryEvent(attempt + 1, delay, error));
                await Task.Delay(delay, cancellationToken);
            }
        }
    }

    private static RetryPolicy Normalize(RetryPolicy policy)
    {
        var initial = policy.InitialBackoff > TimeSpan.Zero ? policy.InitialBackoff : TimeSpan.FromMilliseconds(100);
        return policy with
        {
            MaxAttempts = Math.Max(1, policy.MaxAttempts),
            InitialBackoff = initial,
            MaxBackoff = policy.MaxBackoff >= initial ? policy.MaxBackoff : initial,
            Jitter = Math.Clamp(policy.Jitter, 0, 1),
        };
    }

    private static bool Retryable(RpcException error) => error.StatusCode is
        StatusCode.ResourceExhausted or StatusCode.Unavailable or StatusCode.DeadlineExceeded;

    private static TimeSpan RetryDelay(RpcException error, RetryPolicy policy, int attempt)
    {
        var guided = RetryInfoDelay(error);
        if (guided is { } retryAfter && retryAfter > TimeSpan.Zero)
        {
            return retryAfter;
        }

        var backoffMs = Math.Min(
            policy.MaxBackoff.TotalMilliseconds,
            policy.InitialBackoff.TotalMilliseconds * Math.Pow(2, attempt - 1));
        var factor = 1 - policy.Jitter + Random.Shared.NextDouble() * 2 * policy.Jitter;
        return TimeSpan.FromMilliseconds(Math.Max(1, backoffMs * factor));
    }

    private static TimeSpan? RetryInfoDelay(RpcException error)
    {
        var encoded = error.Trailers.GetValueBytes("grpc-status-details-bin");
        if (encoded is null)
        {
            return null;
        }

        try
        {
            var status = Google.Rpc.Status.Parser.ParseFrom(encoded);
            foreach (var detail in status.Details)
            {
                if (detail.Is(Google.Rpc.RetryInfo.Descriptor))
                {
                    return detail.Unpack<Google.Rpc.RetryInfo>().RetryDelay?.ToTimeSpan();
                }
            }
        }
        catch (InvalidProtocolBufferException)
        {
            // Ignore malformed proxy trailers and fall back to local backoff.
        }
        return null;
    }

    private static string NormalizeAddress(string address)
    {
        if (address.StartsWith("http://", StringComparison.OrdinalIgnoreCase) ||
            address.StartsWith("https://", StringComparison.OrdinalIgnoreCase))
        {
            return address;
        }
        return "http://" + address;
    }

    internal static UsageEventInput EventInput(Event usageEvent)
    {
        var input = new UsageEventInput
        {
            IdempotencyKey = usageEvent.IdempotencyKey,
            Subject = usageEvent.Subject,
            Meter = usageEvent.Meter,
            Quantity = usageEvent.Quantity,
            Timestamp = Timestamp.FromDateTimeOffset(usageEvent.Timestamp.ToUniversalTime()),
        };
        foreach (var (key, value) in usageEvent.Metadata)
        {
            input.Metadata[key] = ValueFor(value);
        }
        return input;
    }

    internal static BulkResult Bulk(CreateUsageBulkResponse response)
    {
        return new BulkResult(
            response.AcceptedCount,
            response.DuplicateCount,
            response.FailedCount,
            response.Accepted.Select(Recorded).ToArray(),
            response.Duplicates.Select(Recorded).ToArray(),
            response.Failed.Select(Failure).ToArray());
    }

    internal static BulkResult Bulk(StreamUsageResponse response)
    {
        return new BulkResult(
            response.AcceptedCount,
            response.DuplicateCount,
            response.FailedCount,
            response.Accepted.Select(Recorded).ToArray(),
            response.Duplicates.Select(Recorded).ToArray(),
            response.Failed.Select(Failure).ToArray());
    }

    private static RecordedEvent Recorded(UsageEvent usageEvent)
    {
        return new RecordedEvent(
            usageEvent.Id,
            usageEvent.IdempotencyKey,
            usageEvent.Subject,
            usageEvent.Meter,
            usageEvent.Quantity,
            DateTimeOffset(usageEvent.Timestamp),
            DateTimeOffset(usageEvent.ReceivedAt),
            usageEvent.Metadata.ToDictionary(item => item.Key, item => ObjectFor(item.Value)));
    }

    private static Failure Failure(BulkFailure failure)
    {
        return new Failure(failure.Index, failure.Code, failure.Message);
    }

    private static DateTimeOffset? DateTimeOffset(Timestamp? value)
    {
        if (value is null || value.Seconds == 0 && value.Nanos == 0)
        {
            return null;
        }
        return value.ToDateTimeOffset();
    }

    private static Value ValueFor(object? value)
    {
        return value switch
        {
            null => new Value { NullValue = NullValue.NullValue },
            string item => new Value { StringValue = item },
            bool item => new Value { BoolValue = item },
            byte item => new Value { NumberValue = item },
            short item => new Value { NumberValue = item },
            int item => new Value { NumberValue = item },
            long item => new Value { NumberValue = item },
            float item => new Value { NumberValue = item },
            double item => new Value { NumberValue = item },
            decimal item => new Value { NumberValue = decimal.ToDouble(item) },
            IReadOnlyDictionary<string, object?> item => new Value { StructValue = StructFor(item) },
            IDictionary<string, object?> item => new Value { StructValue = StructFor(item) },
            IEnumerable<object?> item => new Value { ListValue = ListFor(item) },
            _ => new Value { StringValue = value.ToString() ?? "" },
        };
    }

    private static Struct StructFor(IEnumerable<KeyValuePair<string, object?>> fields)
    {
        var result = new Struct();
        foreach (var (key, value) in fields)
        {
            result.Fields[key] = ValueFor(value);
        }
        return result;
    }

    private static ListValue ListFor(IEnumerable<object?> values)
    {
        var result = new ListValue();
        result.Values.AddRange(values.Select(ValueFor));
        return result;
    }

    private static object? ObjectFor(Value value)
    {
        return value.KindCase switch
        {
            Value.KindOneofCase.NullValue => null,
            Value.KindOneofCase.NumberValue => value.NumberValue,
            Value.KindOneofCase.StringValue => value.StringValue,
            Value.KindOneofCase.BoolValue => value.BoolValue,
            Value.KindOneofCase.StructValue => value.StructValue.Fields.ToDictionary(item => item.Key, item => ObjectFor(item.Value)),
            Value.KindOneofCase.ListValue => value.ListValue.Values.Select(ObjectFor).ToArray(),
            _ => null,
        };
    }
}

public sealed class UsageStream
{
    private readonly AsyncClientStreamingCall<StreamUsageRequest, StreamUsageResponse> call;
    private bool closed;

    internal UsageStream(AsyncClientStreamingCall<StreamUsageRequest, StreamUsageResponse> call)
    {
        this.call = call;
    }

    public async Task TrackAsync(Event usageEvent, CancellationToken cancellationToken = default)
    {
        if (closed)
        {
            throw new InvalidOperationException("Stream is already closed.");
        }
        await call.RequestStream.WriteAsync(new StreamUsageRequest { Event = StreamClient.EventInput(usageEvent) }, cancellationToken);
    }

    public async Task<BulkResult> CloseAsync()
    {
        if (!closed)
        {
            closed = true;
            await call.RequestStream.CompleteAsync();
        }
        return StreamClient.Bulk(await call.ResponseAsync);
    }
}
