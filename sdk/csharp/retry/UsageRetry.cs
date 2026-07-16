using Microsoft.Kiota.Abstractions;

namespace OpenSpanner.Retrying;

public sealed record RestRetryEvent(int Attempt, TimeSpan Delay, Exception Error);

public sealed record RestRetryPolicy
{
    public int MaxAttempts { get; init; } = 3;
    public TimeSpan InitialBackoff { get; init; } = TimeSpan.FromMilliseconds(100);
    public TimeSpan MaxBackoff { get; init; } = TimeSpan.FromSeconds(5);
    public double Jitter { get; init; } = 0.2;
    public Action<RestRetryEvent>? OnRetry { get; init; }
}

public static class UsageRetry
{
    /// <summary>Retries a generated REST usage call for transport errors, 429, and temporary 5xx responses.</summary>
    public static async Task<T> ExecuteAsync<T>(
        Func<CancellationToken, Task<T>> call,
        RestRetryPolicy? policy = null,
        CancellationToken cancellationToken = default)
    {
        var normalized = Normalize(policy ?? new RestRetryPolicy());
        for (var attempt = 1; ; attempt++)
        {
            try
            {
                return await call(cancellationToken);
            }
            catch (Exception error) when (attempt < normalized.MaxAttempts && Retryable(error, cancellationToken))
            {
                var delay = RetryDelay(error, normalized, attempt);
                normalized.OnRetry?.Invoke(new RestRetryEvent(attempt + 1, delay, error));
                await Task.Delay(delay, cancellationToken);
            }
        }
    }

    private static RestRetryPolicy Normalize(RestRetryPolicy policy)
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

    private static bool Retryable(Exception error, CancellationToken cancellationToken) => error switch
    {
        ApiException api => api.ResponseStatusCode is 429 or 500 or 502 or 503 or 504,
        HttpRequestException => true,
        TaskCanceledException when !cancellationToken.IsCancellationRequested => true,
        _ => false,
    };

    private static TimeSpan RetryDelay(Exception error, RestRetryPolicy policy, int attempt)
    {
        if (error is ApiException api && RetryAfter(api) is { } guided && guided > TimeSpan.Zero)
        {
            return guided;
        }
        var backoffMs = Math.Min(policy.MaxBackoff.TotalMilliseconds, policy.InitialBackoff.TotalMilliseconds * Math.Pow(2, attempt - 1));
        var factor = 1 - policy.Jitter + Random.Shared.NextDouble() * 2 * policy.Jitter;
        return TimeSpan.FromMilliseconds(Math.Max(1, backoffMs * factor));
    }

    private static TimeSpan? RetryAfter(ApiException error)
    {
        if (!error.ResponseHeaders.TryGetValue("Retry-After", out var values))
        {
            return null;
        }
        var value = values.FirstOrDefault()?.Trim();
        if (double.TryParse(value, out var seconds) && seconds > 0)
        {
            return TimeSpan.FromSeconds(seconds);
        }
        if (DateTimeOffset.TryParse(value, out var at) && at > DateTimeOffset.UtcNow)
        {
            return at - DateTimeOffset.UtcNow;
        }
        return null;
    }
}
