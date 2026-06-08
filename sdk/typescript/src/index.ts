import type { components, paths } from "./schema.js";

export interface OpenSpannerClientOptions {
  baseUrl: string;
  fetch?: typeof fetch;
  headers?: HeadersInit;
}

export type ErrorBody = components["schemas"]["ErrorBody"];
export type ErrorResponse = components["schemas"]["ErrorResponse"];
export type Meter = components["schemas"]["Meter"];
export type MeterCreateRequest = components["schemas"]["MeterCreateRequest"];
export type MeterUpdateRequest = components["schemas"]["MeterUpdateRequest"];
export type MeterListResponse = components["schemas"]["MeterListResponse"];
export type UsageCreateRequest = components["schemas"]["UsageCreateRequest"];
export type UsageEvent = components["schemas"]["UsageEvent"];
export type UsageBulkResult = components["schemas"]["UsageBulkResult"];
export type UsageBucket = components["schemas"]["UsageBucket"];
export type UsageBucketListResponse =
  paths["/v1/usages"]["get"]["responses"]["200"]["content"]["application/json"];
export type ListMetersParams = NonNullable<paths["/v1/meters"]["get"]["parameters"]["query"]>;
export type ListUsageBucketsParams = NonNullable<paths["/v1/usages"]["get"]["parameters"]["query"]>;

export class OpenSpannerError extends Error {
  readonly status: number;
  readonly body: unknown;

  constructor(status: number, body: unknown) {
    super(`Open Spanner request failed with status ${status}`);
    this.name = "OpenSpannerError";
    this.status = status;
    this.body = body;
  }
}

export class OpenSpannerClient {
  private readonly baseUrl: string;
  private readonly fetchFn: typeof fetch;
  private readonly headers?: HeadersInit;

  constructor(options: OpenSpannerClientOptions) {
    this.baseUrl = options.baseUrl.replace(/\/+$/, "");
    this.fetchFn = options.fetch ?? fetch;
    this.headers = options.headers;
  }

  health(): Promise<void> {
    return this.request<void>("GET", "/health");
  }

  createMeter(input: MeterCreateRequest): Promise<Meter> {
    return this.request<Meter>("POST", "/v1/meters", input);
  }

  listMeters(params: ListMetersParams = {}): Promise<MeterListResponse> {
    return this.request<MeterListResponse>("GET", `/v1/meters${toQuery(params)}`);
  }

  getMeter(name: string): Promise<Meter> {
    return this.request<Meter>("GET", `/v1/meters/${encodeURIComponent(name)}`);
  }

  updateMeter(name: string, input: MeterUpdateRequest): Promise<Meter> {
    return this.request<Meter>("PUT", `/v1/meters/${encodeURIComponent(name)}`, input);
  }

  deleteMeter(name: string): Promise<void> {
    return this.request<void>("DELETE", `/v1/meters/${encodeURIComponent(name)}`);
  }

  createUsage(input: UsageCreateRequest): Promise<UsageEvent> {
    return this.request<UsageEvent>("POST", "/v1/usages", input);
  }

  createUsageBulk(input: UsageCreateRequest[]): Promise<UsageBulkResult> {
    return this.request<UsageBulkResult>("POST", "/v1/usages/bulk", input);
  }

  listUsageBuckets(params: ListUsageBucketsParams): Promise<UsageBucketListResponse> {
    return this.request<UsageBucketListResponse>("GET", `/v1/usages${toQuery(params)}`);
  }

  private async request<T>(method: string, path: string, body?: unknown): Promise<T> {
    const response = await this.fetchFn(`${this.baseUrl}${path}`, {
      body: body === undefined ? undefined : JSON.stringify(body),
      headers: {
        ...this.headers,
        ...(body === undefined ? {} : { "Content-Type": "application/json" }),
      },
      method,
    });

    if (!response.ok) {
      throw new OpenSpannerError(response.status, await readBody(response));
    }

    if (response.status === 204) {
      return undefined as T;
    }

    return (await response.json()) as T;
  }
}

function toQuery(params: object): string {
  const query = new URLSearchParams();
  for (const [key, value] of Object.entries(params) as Array<[string, unknown]>) {
    if (value !== undefined && value !== null && value !== "") {
      query.set(key, String(value));
    }
  }
  const encoded = query.toString();
  return encoded ? `?${encoded}` : "";
}

async function readBody(response: Response): Promise<unknown> {
  const text = await response.text();
  if (!text) {
    return undefined;
  }
  try {
    return JSON.parse(text) as unknown;
  } catch {
    return text;
  }
}
