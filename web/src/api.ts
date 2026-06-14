export type SystemStats = {
  meters: number
  usage_events: number
  prune_runs: number
  last_prune_run: null | {
    id: string
    deleted: number
    dry_run: boolean
    created_at: string
  }
}

export type SubjectStats = {
  subject: string
  usage_events: number
  meters: number
  last_event_at: string
}

export type SubjectList = {
  items: SubjectStats[]
  next_cursor?: string
}

export type IngestionRun = {
  id: string
  kind: string
  accepted: number
  duplicates: number
  failed: number
  created_at: string
}

export type IngestionList = {
  items: IngestionRun[]
  next_cursor?: string
}

export type Meter = {
  id: string
  name: string
  description: string
  unit: string
  aggregation: string
  metadata_schema: Record<string, string>
  event_retention_days: number
  created_at: string
}

export type MeterList = {
  items: Meter[]
  next_cursor?: string
}

export type MeterStats = {
  meter: string
  usage_events: number
  last_event_at?: string
  retention_days: number
}

export type MeterStatsList = {
  items: MeterStats[]
  next_cursor?: string
}

export type MeterCreateRequest = {
  name: string
  description: string
  unit: string
  aggregation: string
  metadata_schema: Record<string, string>
  event_retention_days: number
}

export type MeterUpdateRequest = {
  description: string
}

export type UsageEvent = {
  id: string
  idempotency_key?: string
  subject: string
  meter: string
  quantity: number
  timestamp: string
  received_at: string
  metadata: Record<string, unknown>
}

export type UsageCreateRequest = {
  idempotency_key: string
  subject: string
  meter: string
  quantity: number
  timestamp: string
  metadata: Record<string, unknown>
}

export type UsageBucket = {
  subject: string
  meter: string
  bucket_size: string
  bucket_start: string
  aggregation: string
  unit: string
  quantity: number
  group?: Record<string, string>
}

export type UsageBucketQuery = {
  subject?: string
  meter?: string
  from: string
  to: string
  bucket_size: string
  group_by?: string
  limit?: number
  filter?: UsageFilter
}

export type UsageFilter = UsageFilterGroup | UsageFilterCondition

export type UsageFilterGroup = {
  type: 'group'
  op: 'and' | 'or'
  rules: UsageFilter[]
}

export type UsageFilterCondition = {
  type: 'condition'
  field: string
  op: 'eq' | 'neq' | 'gt' | 'gte' | 'lt' | 'lte' | 'in' | 'contains' | 'exists'
  value?: unknown
}

export type AuthUser = {
  id: string
  email: string
  created_at: string
}

export type AuthSession = {
  expires_at: string
  user: AuthUser
}

export type CurrentAuthSession = {
  user: AuthUser
}

let currentAuthUser: AuthUser | null = null

export function readAuthUser() {
  return currentAuthUser
}

export function setAuthUser(user: AuthUser | null) {
  currentAuthUser = user
}

export async function loadAuthUser() {
  const response = await fetch('/v1/auth/session', {
    credentials: 'same-origin',
  })

  if (response.status === 401) {
    currentAuthUser = null
    return null
  }
  if (!response.ok) {
    const payload = await response.json().catch(() => ({ error: { message: response.statusText } }))
    const error = typeof payload.error === 'string' ? payload.error : payload.error?.message
    throw new Error(error || response.statusText)
  }

  const session = await response.json() as CurrentAuthSession
  currentAuthUser = session.user
  return session.user
}

async function request<T>(path: string, options: RequestInit = {}): Promise<T> {
  const headers = new Headers(options.headers)
  if (options.body && !headers.has('Content-Type')) {
    headers.set('Content-Type', 'application/json')
  }

  const response = await fetch(path, {
    ...options,
    credentials: 'same-origin',
    headers,
  })

  if (!response.ok) {
    if (response.status === 401) {
      currentAuthUser = null
    }
    const payload = await response.json().catch(() => ({ error: { message: response.statusText } }))
    const error = typeof payload.error === 'string' ? payload.error : payload.error?.message
    throw new Error(error || response.statusText)
  }

  if (response.status === 204) {
    return undefined as T
  }

  return response.json() as Promise<T>
}

export async function createAuthSession(input: { email: string; password: string }) {
  const session = await request<AuthSession>('/v1/auth/sessions', {
    body: JSON.stringify(input),
    method: 'POST',
  })
  currentAuthUser = session.user
  return session
}

export async function deleteAuthSession() {
  await request<void>('/v1/auth/session', {
    method: 'DELETE',
  })
  currentAuthUser = null
}

export async function getSystemStats() {
  return request<SystemStats>('/v1/system/stats')
}

export async function listSubjects(limit = 8) {
  return request<SubjectList>(`/v1/subjects?limit=${limit}`)
}

export async function listIngestions(limit = 8) {
  return request<IngestionList>(`/v1/usageingestions?limit=${limit}`)
}

export async function listMeters(limit = 100) {
  return request<MeterList>(`/v1/meters?limit=${limit}`)
}

export async function listMeterStats(limit = 100) {
  return request<MeterStatsList>(`/v1/meters/stats?limit=${limit}`)
}

export async function createMeter(input: MeterCreateRequest) {
  return request<Meter>('/v1/meters', {
    body: JSON.stringify(input),
    method: 'POST',
  })
}

export async function updateMeter(id: string, input: MeterUpdateRequest) {
  return request<Meter>(`/v1/meters/${encodeURIComponent(id)}`, {
    body: JSON.stringify(input),
    method: 'PUT',
  })
}

export async function deleteMeter(id: string) {
  return request<void>(`/v1/meters/${encodeURIComponent(id)}`, {
    method: 'DELETE',
  })
}

export async function createUsage(input: UsageCreateRequest) {
  return request<UsageEvent>('/v1/usages', {
    body: JSON.stringify(input),
    method: 'POST',
  })
}

export async function listUsageBuckets(query: UsageBucketQuery) {
  return request<UsageBucket[]>('/v1/usages/search', {
    body: JSON.stringify({
      bucket_size: query.bucket_size,
      filter: query.filter,
      from: query.from,
      group_by: query.group_by || undefined,
      limit: query.limit,
      meter: query.meter,
      subject: query.subject,
      to: query.to,
    }),
    method: 'POST',
  })
}
