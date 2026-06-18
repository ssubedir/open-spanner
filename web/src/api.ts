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
  deprecated: boolean
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
  deprecated: boolean
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
  filter?: UsageFilter
}

export type UsageEventExportQuery = {
  subject?: string
  meter?: string
  from?: string
  to?: string
  limit?: number
  filter?: UsageFilter
}

export type UsageEventQuery = UsageEventExportQuery & {
  cursor?: string
}

export type UsageEventList = {
  items: UsageEvent[]
  next_cursor?: string
}

export type UsageExportJob = {
  id: string
  kind: string
  status: string
  format: string
  query: UsageBucketExportQuery
  error?: string
  artifact_size?: number
  download_url?: string
  created_at: string
  updated_at: string
  completed_at?: string
}

export type UsageExportJobList = {
  items: UsageExportJob[]
  next_cursor?: string
}

export type UsageExportJobCreateRequest = {
  kind: string
  format: string
  query: UsageBucketExportQuery
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

export type AlertState = {
  status: string
  group_key?: string
  group_value?: string
  value: number
  message: string
  evaluated_at?: string
  updated_at: string
}

export type AlertRule = {
  id: string
  name: string
  meter: string
  enabled: boolean
  subject?: string
  metadata?: Record<string, string>
  window_seconds: number
  comparator: string
  threshold: number
  evaluation_interval_seconds: number
  group_by?: string
  trigger_type: string
  webhook_url?: string
  next_evaluate_at: string
  created_at: string
  updated_at: string
  state?: AlertState
  states?: AlertState[]
}

export type AlertRuleList = {
  items: AlertRule[]
}

export type AlertEvent = {
  id: string
  rule_id: string
  group_key?: string
  group_value?: string
  type: string
  value: number
  message: string
  created_at: string
}

export type AlertEventList = {
  items: AlertEvent[]
  next_cursor?: string
}

export type AlertRuleRequest = {
  name: string
  meter: string
  enabled?: boolean
  subject?: string
  metadata?: Record<string, string>
  window_seconds?: number
  comparator?: string
  threshold: number
  evaluation_interval_seconds?: number
  group_by?: string
  trigger_type?: string
  webhook_url?: string
}

export type AlertRuleUpdateRequest = Partial<Omit<AlertRuleRequest, 'threshold'>> & {
  threshold?: number
}

export class APIError extends Error {
  code: string
  status: number

  constructor(message: string, status: number, code: string) {
    super(message)
    this.name = 'APIError'
    this.code = code
    this.status = status
  }
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
    throw await apiError(response)
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
    throw await apiError(response)
  }

  if (response.status === 204) {
    return undefined as T
  }

  return response.json() as Promise<T>
}

async function requestBlob(path: string, options: RequestInit = {}) {
  const headers = new Headers(options.headers)
  if (options.body && !headers.has('Content-Type')) {
    headers.set('Content-Type', 'application/json')
  }

  const response = await fetchWithAuthRefresh(path, {
    ...options,
    headers,
  })

  if (!response.ok) {
    throw await apiError(response)
  }

  return response.blob()
}

export async function createAuthSession(input: { email: string; password: string }) {
  return request<AuthSession>('/v1/auth/sessions', {
    body: JSON.stringify(input),
    method: 'POST',
  })
}

async function apiError(response: Response) {
  const payload = await response.json().catch(() => ({ error: { code: '', message: response.statusText } }))
  const message = typeof payload.error === 'string' ? payload.error : payload.error?.message
  const code = typeof payload.error === 'string' ? '' : payload.error?.code

  return new APIError(message || response.statusText, response.status, code || '')
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

export async function listAlertRules() {
  return request<AlertRuleList>('/v1/alerts')
}

export async function createAlertRule(input: AlertRuleRequest) {
  return request<AlertRule>('/v1/alerts', {
    body: JSON.stringify(input),
    method: 'POST',
  })
}

export async function updateAlertRule(id: string, input: AlertRuleUpdateRequest) {
  return request<AlertRule>(`/v1/alerts/${encodeURIComponent(id)}`, {
    body: JSON.stringify(input),
    method: 'PUT',
  })
}

export async function deleteAlertRule(id: string) {
  return request<void>(`/v1/alerts/${encodeURIComponent(id)}`, {
    method: 'DELETE',
  })
}

export async function evaluateAlertRule(id: string) {
  return request<{ rule: AlertRule; state: AlertState; event?: AlertEvent; events?: AlertEvent[] }>(`/v1/alerts/${encodeURIComponent(id)}/evaluate`, {
    method: 'POST',
  })
}

export async function listAlertEvents(limit = 25) {
  return request<AlertEventList>(`/v1/alerts/events?limit=${limit}`)
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
  return requestBlob('/v1/usages/export', {
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

export async function exportUsageEvents(query: UsageEventExportQuery) {
  return requestBlob('/v1/usageevents/export', {
    body: JSON.stringify({
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

export async function listUsageEvents(query: UsageEventQuery) {
  return request<UsageEventList>('/v1/usageevents/search', {
    body: JSON.stringify({
      cursor: query.cursor,
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

export async function createUsageExportJob(input: UsageExportJobCreateRequest) {
  return request<UsageExportJob>('/v1/exports', {
    body: JSON.stringify(input),
    method: 'POST',
  })
}

export async function cancelUsageExportJob(id: string) {
  return request<UsageExportJob>(`/v1/exports/${encodeURIComponent(id)}/cancel`, {
    method: 'POST',
  })
}

export async function retryUsageExportJob(id: string) {
  return request<UsageExportJob>(`/v1/exports/${encodeURIComponent(id)}/retry`, {
    method: 'POST',
  })
}

export async function listUsageExportJobs(limit = 8) {
  return request<UsageExportJobList>(`/v1/exports?limit=${limit}`)
}

export async function downloadUsageExportJob(job: Pick<UsageExportJob, 'download_url' | 'id'>) {
  return requestBlob(job.download_url || `/v1/exports/${encodeURIComponent(job.id)}/download`)
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
