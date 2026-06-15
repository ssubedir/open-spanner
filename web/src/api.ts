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
  dimensions: MeterDimension[]
  metadata_schema: Record<string, string>
  event_retention_days: number
  created_at: string
}

export type MeterDimension = {
  name: string
  display_name: string
  description: string
  type: string
  required: boolean
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
  dimensions: MeterDimensionRequest[]
  metadata_schema: Record<string, string>
  event_retention_days: number
}

export type MeterUpdateRequest = {
  description: string
  unit: string
  aggregation: string
  dimensions: MeterDimensionRequest[]
  metadata_schema: Record<string, string>
  event_retention_days: number
}

export type MeterDimensionRequest = {
  name: string
  display_name: string
  description: string
  type: string
  required: boolean
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
  group_by?: string[]
  limit?: number
  filter?: UsageFilter
}

export type UsageBucketExportQuery = {
  subject?: string
  meter: string
  from: string
  to: string
  bucket_size: string
  group_by?: string[]
  limit?: number
  metadata?: Record<string, string>
}

export type UsageEventExportQuery = {
  subject?: string
  meter?: string
  from?: string
  to?: string
  limit?: number
}

export type UsageDimensionValue = {
  field: string
  value: string
  events: number
}

export type UsageDimensionValueList = {
  items: UsageDimensionValue[]
}

export type UsageBreakdown = {
  field: string
  value: string
  quantity: number
  events: number
  aggregation: string
  unit: string
}

export type UsageBreakdownList = {
  items: UsageBreakdown[]
}

export type UsageDimensionValueQuery = {
  meter: string
  field: string
  subject?: string
  from?: string
  to?: string
  limit?: number
}

export type UsageBreakdownQuery = {
  subject?: string
  meter: string
  field: string
  from: string
  to: string
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

export type SavedUsageQuery = {
  id: string
  name: string
  query: unknown
  group_by: string[]
  bucket_size: string
  limit: number
  pinned: boolean
  position: number
  created_at: string
  updated_at: string
}

export type SavedUsageQueryList = {
  items: SavedUsageQuery[]
}

export type SavedUsageQueryRequest = {
  name: string
  query: unknown
  group_by: string[]
  bucket_size: string
  limit: number
  pinned: boolean
  position: number
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

export type APIKey = {
  id: string
  name: string
  prefix: string
  created_at: string
  last_used_at: string | null
}

export type APIKeyList = {
  items: APIKey[]
}

export type APIKeyCreateResponse = APIKey & {
  key: string
}

export async function createAuthUser(input: { email: string; password: string }) {
  return request<AuthUser>('/v1/auth/users', {
    body: JSON.stringify(input),
    method: 'POST',
  })
}

export async function refreshAuthSession() {
  const response = await fetch('/v1/auth/session/refresh', {
    credentials: 'same-origin',
    method: 'POST',
  })

  if (response.status === 401) {
    return null
  }
  if (!response.ok) {
    const payload = await response.json().catch(() => ({ error: { message: response.statusText } }))
    const error = typeof payload.error === 'string' ? payload.error : payload.error?.message
    throw new Error(error || response.statusText)
  }

  return response.json() as Promise<AuthSession>
}

async function fetchWithAuthRefresh(path: string, options: RequestInit = {}, retry = true) {
  const response = await fetch(path, {
    ...options,
    credentials: 'same-origin',
  })

  if (response.status === 401) {
    if (retry && path !== '/v1/auth/session/refresh' && await refreshAuthSession()) {
      return fetchWithAuthRefresh(path, options, false)
    }
  }
  return response
}

async function request<T>(path: string, options: RequestInit = {}): Promise<T> {
  const headers = new Headers(options.headers)
  if (options.body && !headers.has('Content-Type')) {
    headers.set('Content-Type', 'application/json')
  }

  const response = await fetchWithAuthRefresh(path, {
    ...options,
    headers,
  })

  if (!response.ok) {
    const payload = await response.json().catch(() => ({ error: { message: response.statusText } }))
    const error = typeof payload.error === 'string' ? payload.error : payload.error?.message
    throw new Error(error || response.statusText)
  }

  if (response.status === 204) {
    return undefined as T
  }

  return response.json() as Promise<T>
}

async function requestBlob(path: string, options: RequestInit = {}) {
  const response = await fetchWithAuthRefresh(path, options)

  if (!response.ok) {
    const payload = await response.json().catch(() => ({ error: { message: response.statusText } }))
    const error = typeof payload.error === 'string' ? payload.error : payload.error?.message
    throw new Error(error || response.statusText)
  }

  return response.blob()
}

export async function createAuthSession(input: { email: string; password: string }) {
  return request<AuthSession>('/v1/auth/sessions', {
    body: JSON.stringify(input),
    method: 'POST',
  })
}

export async function deleteAuthSession() {
  await request<void>('/v1/auth/session', {
    method: 'DELETE',
  })
}

export async function listAPIKeys() {
  return request<APIKeyList>('/v1/auth/api-keys')
}

export async function createAPIKey(input: { name: string }) {
  return request<APIKeyCreateResponse>('/v1/auth/api-keys', {
    body: JSON.stringify(input),
    method: 'POST',
  })
}

export async function deleteAPIKey(id: string) {
  return request<void>(`/v1/auth/api-keys/${encodeURIComponent(id)}`, {
    method: 'DELETE',
  })
}

export async function getSystemStats() {
  return request<SystemStats>('/v1/system/stats')
}

export async function listSubjects(limit = 8) {
  return request<SubjectList>(`/v1/subjects?limit=${limit}`)
}

export async function listSubjectEvents(subject: string, limit = 25) {
  return request<UsageEvent[]>(`/v1/subjects/${encodeURIComponent(subject)}/usageevents?limit=${limit}`)
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
      group_by: query.group_by && query.group_by.length > 0 ? query.group_by : undefined,
      limit: query.limit,
      meter: query.meter,
      subject: query.subject,
      to: query.to,
    }),
    method: 'POST',
  })
}

export async function exportUsageBuckets(query: UsageBucketExportQuery) {
  const params = new URLSearchParams({
    bucket_size: query.bucket_size,
    from: query.from,
    meter: query.meter,
    to: query.to,
  })
  if (query.subject) {
    params.set('subject', query.subject)
  }
  if (query.limit) {
    params.set('limit', String(query.limit))
  }
  query.group_by?.forEach((field) => {
    if (field) {
      params.append('group_by', field)
    }
  })
  Object.entries(query.metadata || {}).forEach(([key, value]) => {
    if (key && value !== '') {
      params.set(`metadata.${key}`, value)
    }
  })

  return requestBlob(`/v1/usages/export?${params.toString()}`)
}

export async function exportUsageEvents(query: UsageEventExportQuery) {
  const params = new URLSearchParams()
  if (query.subject) {
    params.set('subject', query.subject)
  }
  if (query.meter) {
    params.set('meter', query.meter)
  }
  if (query.from) {
    params.set('from', query.from)
  }
  if (query.to) {
    params.set('to', query.to)
  }
  if (query.limit) {
    params.set('limit', String(query.limit))
  }

  return requestBlob(`/v1/usageevents/export?${params.toString()}`)
}

export async function listUsageDimensionValues(query: UsageDimensionValueQuery) {
  const params = new URLSearchParams({
    field: query.field,
    meter: query.meter,
  })
  if (query.subject) {
    params.set('subject', query.subject)
  }
  if (query.from) {
    params.set('from', query.from)
  }
  if (query.to) {
    params.set('to', query.to)
  }
  if (query.limit) {
    params.set('limit', String(query.limit))
  }
  return request<UsageDimensionValueList>(`/v1/usages/dimensions?${params.toString()}`)
}

export async function listUsageBreakdowns(query: UsageBreakdownQuery) {
  return request<UsageBreakdownList>('/v1/usages/breakdowns/search', {
    body: JSON.stringify({
      field: query.field,
      filter: query.filter,
      from: query.from,
      limit: query.limit,
      meter: query.meter,
      subject: query.subject,
      to: query.to,
    }),
    method: 'POST',
  })
}

export async function listSavedUsageQueries() {
  return request<SavedUsageQueryList>('/v1/usage/saved-queries')
}

export async function createSavedUsageQuery(input: SavedUsageQueryRequest) {
  return request<SavedUsageQuery>('/v1/usage/saved-queries', {
    body: JSON.stringify(input),
    method: 'POST',
  })
}

export async function updateSavedUsageQuery(id: string, input: SavedUsageQueryRequest) {
  return request<SavedUsageQuery>(`/v1/usage/saved-queries/${encodeURIComponent(id)}`, {
    body: JSON.stringify(input),
    method: 'PUT',
  })
}

export async function deleteSavedUsageQuery(id: string) {
  return request<void>(`/v1/usage/saved-queries/${encodeURIComponent(id)}`, {
    method: 'DELETE',
  })
}
