export interface RestRetryOptions {
  maxAttempts?: number;
  initialBackoffMs?: number;
  maxBackoffMs?: number;
  jitter?: number;
  onRetry?: (event: RestRetryEvent) => void;
}

export interface RestRetryEvent {
  attempt: number;
  delayMs: number;
  error?: unknown;
  response?: Response;
}

type SDKResult = { response?: Response };

/** Retries a generated REST usage call with server-directed backpressure. */
export async function retryUsageWrite<T extends SDKResult>(call: () => Promise<T>, options: RestRetryOptions = {}): Promise<T> {
  const policy = normalize(options);
  for (let attempt = 1; ; attempt++) {
    let result: T | undefined;
    let error: unknown;
    try {
      result = await call();
      if (!retryableStatus(result.response?.status)) {
        return result;
      }
    } catch (caught) {
      if (!(caught instanceof TypeError)) {
        throw caught;
      }
      error = caught;
    }
    if (attempt >= policy.maxAttempts) {
      if (error !== undefined) throw error;
      return result!;
    }
    const delayMs = retryDelay(result?.response, policy, attempt);
    policy.onRetry?.({ attempt: attempt + 1, delayMs, error, response: result?.response });
    await new Promise((resolve) => setTimeout(resolve, delayMs));
  }
}

function normalize(options: RestRetryOptions) {
  const initialBackoffMs = Math.max(1, options.initialBackoffMs ?? 100);
  return {
    maxAttempts: Math.max(1, Math.floor(options.maxAttempts ?? 3)),
    initialBackoffMs,
    maxBackoffMs: Math.max(initialBackoffMs, options.maxBackoffMs ?? 5_000),
    jitter: Math.min(1, Math.max(0, options.jitter ?? 0.2)),
    onRetry: options.onRetry,
  };
}

function retryableStatus(status: number | undefined): boolean {
  return status === 429 || status === 500 || status === 502 || status === 503 || status === 504;
}

function retryDelay(response: Response | undefined, policy: ReturnType<typeof normalize>, attempt: number): number {
  const retryAfter = response?.headers.get("Retry-After")?.trim();
  if (retryAfter) {
    const seconds = Number(retryAfter);
    if (Number.isFinite(seconds) && seconds > 0) return Math.ceil(seconds * 1_000);
    const date = Date.parse(retryAfter);
    if (Number.isFinite(date) && date > Date.now()) return date - Date.now();
  }
  const backoff = Math.min(policy.maxBackoffMs, policy.initialBackoffMs * 2 ** (attempt - 1));
  return Math.max(1, Math.round(backoff * (1 - policy.jitter + Math.random() * 2 * policy.jitter)));
}
