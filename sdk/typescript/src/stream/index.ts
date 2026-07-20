import {
  credentials,
  Metadata,
  status,
  type ChannelCredentials,
  type ClientOptions,
  type ClientWritableStream,
  type ServiceError,
} from "@grpc/grpc-js";

import {
  UsageServiceClient,
  type BulkFailure,
  type CreateUsageBulkResponse,
  type StreamUsageRequest,
  type StreamUsageResponse,
  type UsageEvent,
  type UsageEventInput,
} from "../grpc/pb/open_spanner/v1/usage.js";

export interface StreamClientOptions {
  credentials?: ChannelCredentials;
  clientOptions?: Partial<ClientOptions>;
  retry?: RetryOptions;
}

export interface RetryOptions {
  /** Total attempts including the initial request. */
  maxAttempts?: number;
  initialBackoffMs?: number;
  maxBackoffMs?: number;
  /** Fractional randomization from 0 to 1. */
  jitter?: number;
  onRetry?: (event: RetryEvent) => void;
}

export interface RetryEvent {
  attempt: number;
  delayMs: number;
  error: ServiceError;
}

export interface Event {
  idempotencyKey?: string;
  subject: string;
  meter: string;
  quantity: number;
  timestamp?: Date;
  metadata?: Record<string, unknown>;
}

export type RecordedEvent = UsageEvent;
export type Failure = BulkFailure;
export type BulkResult = CreateUsageBulkResponse;

export class StreamClient {
  private readonly client: UsageServiceClient;
  private readonly apiKey: string;
  private readonly retry?: Required<Omit<RetryOptions, "onRetry">> & Pick<RetryOptions, "onRetry">;

  constructor(address: string, apiKey: string, options: StreamClientOptions = {}) {
    const normalizedAddress = address.trim();
    if (normalizedAddress === "") {
      throw new Error("gRPC address is required");
    }

    const normalizedApiKey = apiKey.trim();
    if (normalizedApiKey === "") {
      throw new Error("API key is required");
    }

    this.apiKey = normalizedApiKey;
    this.retry = options.retry ? normalizeRetryOptions(options.retry) : undefined;
    this.client = new UsageServiceClient(
      normalizedAddress,
      options.credentials ?? credentials.createInsecure(),
      options.clientOptions,
    );
  }

  close(): void {
    (this.client as unknown as { close(): void }).close();
  }

  async track(event: Event): Promise<RecordedEvent> {
    const request = { event: eventInput(event) };
    const response = await unary<UsageEvent | undefined>((metadata, callback) => {
      this.client.createUsage(request, metadata, (error, result) => {
        callback(error, result?.event);
      });
    }, this.authMetadata(), this.retry);

    if (!response) {
      throw new Error("gRPC response did not include a usage event");
    }

    return response;
  }

  trackBulk(idempotencyKey: string, events: Event[]): Promise<BulkResult> {
    const request = {
      idempotencyKey,
      events: events.map(eventInput),
    };
    return unary((metadata, callback) => {
      this.client.createUsageBulk(request, metadata, callback);
    }, this.authMetadata(), this.retry);
  }

  stream(idempotencyKey: string): UsageStream {
    let rejectResult!: (error: unknown) => void;
    let resolveResult!: (result: StreamUsageResponse) => void;
    const result = new Promise<StreamUsageResponse>((resolve, reject) => {
      resolveResult = resolve;
      rejectResult = reject;
    });

    const stream = this.client.streamUsage(this.authMetadata({ "idempotency-key": idempotencyKey }), (error, result) => {
      if (error) {
        rejectResult(error);
        return;
      }

      resolveResult(result);
    });

    return new UsageStream(stream, result);
  }

  private authMetadata(values: Record<string, string> = {}): Metadata {
    const metadata = new Metadata();
    metadata.set("authorization", `Bearer ${this.apiKey}`);

    for (const [key, value] of Object.entries(values)) {
      metadata.set(key, value);
    }

    return metadata;
  }
}

export class UsageStream {
  private closed = false;

  constructor(
    private readonly writable: ClientWritableStream<StreamUsageRequest>,
    private readonly result: Promise<StreamUsageResponse>,
  ) {}

  track(event: Event): Promise<void> {
    if (this.closed) {
      return Promise.reject(new Error("stream is already closed"));
    }

    return new Promise((resolve, reject) => {
      this.writable.write({ event: eventInput(event) }, (error: Error | null | undefined) => {
        if (error) {
          reject(error);
          return;
        }

        resolve();
      });
    });
  }

  close(): Promise<StreamUsageResponse> {
    if (!this.closed) {
      this.closed = true;
      this.writable.end();
    }

    return this.result;
  }
}

function eventInput(event: Event): UsageEventInput {
  return {
    idempotencyKey: event.idempotencyKey ?? "",
    subject: event.subject,
    meter: event.meter,
    quantity: event.quantity,
    timestamp: event.timestamp ?? new Date(),
    metadata: event.metadata ?? {},
  };
}

function unary<T>(
  call: (metadata: Metadata, callback: (error: ServiceError | null, response: T) => void) => void,
  metadata: Metadata,
  retry?: Required<Omit<RetryOptions, "onRetry">> & Pick<RetryOptions, "onRetry">,
): Promise<T> {
  const invoke = () => new Promise<T>((resolve, reject) => {
    call(metadata, (error, response) => {
      if (error) {
        reject(error);
        return;
      }
      resolve(response);
    });
  });
  if (!retry) {
    return invoke();
  }
  return retryUnary(invoke, retry);
}

function normalizeRetryOptions(options: RetryOptions): Required<Omit<RetryOptions, "onRetry">> & Pick<RetryOptions, "onRetry"> {
  return {
    maxAttempts: Math.max(1, Math.floor(options.maxAttempts ?? 3)),
    initialBackoffMs: Math.max(1, options.initialBackoffMs ?? 100),
    maxBackoffMs: Math.max(options.initialBackoffMs ?? 100, options.maxBackoffMs ?? 5_000),
    jitter: Math.min(1, Math.max(0, options.jitter ?? 0.2)),
    onRetry: options.onRetry,
  };
}

async function retryUnary<T>(
  call: () => Promise<T>,
  retry: Required<Omit<RetryOptions, "onRetry">> & Pick<RetryOptions, "onRetry">,
): Promise<T> {
  for (let attempt = 1; ; attempt++) {
    try {
      return await call();
    } catch (error) {
      const serviceError = error as ServiceError;
      if (attempt >= retry.maxAttempts || !retryableError(serviceError)) {
        throw error;
      }
      const delayMs = retryDelayMs(serviceError, retry, attempt);
      retry.onRetry?.({ attempt: attempt + 1, delayMs, error: serviceError });
      await new Promise((resolve) => setTimeout(resolve, delayMs));
    }
  }
}

function retryableError(error: ServiceError): boolean {
  return error.code === status.RESOURCE_EXHAUSTED || error.code === status.UNAVAILABLE || error.code === status.DEADLINE_EXCEEDED;
}

function retryDelayMs(
  error: ServiceError,
  retry: Required<Omit<RetryOptions, "onRetry">> & Pick<RetryOptions, "onRetry">,
  attempt: number,
): number {
  const guided = retryInfoDelayMs(error);
  if (guided !== undefined && guided > 0) {
    return guided;
  }
  const backoff = Math.min(retry.maxBackoffMs, retry.initialBackoffMs * 2 ** (attempt - 1));
  const factor = 1 - retry.jitter + Math.random() * 2 * retry.jitter;
  return Math.max(1, Math.round(backoff * factor));
}

function retryInfoDelayMs(error: ServiceError): number | undefined {
  for (const value of error.metadata?.get("grpc-status-details-bin") ?? []) {
    const bytes = typeof value === "string" ? Buffer.from(value, "base64") : new Uint8Array(value);
    for (const detail of protobufBytes(bytes, 3)) {
      const type = protobufBytes(detail, 1)[0];
      const payload = protobufBytes(detail, 2)[0];
      if (!type || !payload || !new TextDecoder().decode(type).endsWith("google.rpc.RetryInfo")) {
        continue;
      }
      const duration = protobufBytes(payload, 1)[0];
      if (!duration) {
        continue;
      }
      const seconds = protobufVarint(duration, 1) ?? 0;
      const nanos = protobufVarint(duration, 2) ?? 0;
      return seconds * 1_000 + Math.ceil(nanos / 1_000_000);
    }
  }
  return undefined;
}

function protobufBytes(data: Uint8Array, wantedField: number): Uint8Array[] {
  const results: Uint8Array[] = [];
  for (let offset = 0; offset < data.length;) {
    const tag = readVarint(data, offset);
    if (!tag) break;
    offset = tag.next;
    const field = tag.value >>> 3;
    const wire = tag.value & 7;
    if (wire === 2) {
      const length = readVarint(data, offset);
      if (!length) break;
      offset = length.next;
      const end = offset + length.value;
      if (field === wantedField) results.push(data.subarray(offset, end));
      offset = end;
    } else if (wire === 0) {
      const value = readVarint(data, offset);
      if (!value) break;
      offset = value.next;
    } else {
      break;
    }
  }
  return results;
}

function protobufVarint(data: Uint8Array, wantedField: number): number | undefined {
  for (let offset = 0; offset < data.length;) {
    const tag = readVarint(data, offset);
    if (!tag) return undefined;
    offset = tag.next;
    const field = tag.value >>> 3;
    const wire = tag.value & 7;
    if (wire === 0) {
      const value = readVarint(data, offset);
      if (!value) return undefined;
      if (field === wantedField) return value.value;
      offset = value.next;
    } else if (wire === 2) {
      const length = readVarint(data, offset);
      if (!length) return undefined;
      offset = length.next + length.value;
    } else {
      return undefined;
    }
  }
  return undefined;
}

function readVarint(data: Uint8Array, offset: number): { value: number; next: number } | undefined {
  let value = 0;
  for (let shift = 0; offset < data.length && shift < 53; shift += 7, offset++) {
    const byte = data[offset];
    value += (byte & 0x7f) * 2 ** shift;
    if ((byte & 0x80) === 0) return { value, next: offset + 1 };
  }
  return undefined;
}
