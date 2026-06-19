import {
  credentials,
  Metadata,
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
    const response = await unary<UsageEvent | undefined>((metadata, callback) => {
      this.client.createUsage({ event: eventInput(event) }, metadata, (error, result) => {
        callback(error, result?.event);
      });
    }, this.authMetadata());

    if (!response) {
      throw new Error("gRPC response did not include a usage event");
    }

    return response;
  }

  trackBulk(idempotencyKey: string, events: Event[]): Promise<BulkResult> {
    return unary((metadata, callback) => {
      this.client.createUsageBulk(
        {
          idempotencyKey,
          events: events.map(eventInput),
        },
        metadata,
        callback,
      );
    }, this.authMetadata());
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
): Promise<T> {
  return new Promise((resolve, reject) => {
    call(metadata, (error, response) => {
      if (error) {
        reject(error);
        return;
      }

      resolve(response);
    });
  });
}
