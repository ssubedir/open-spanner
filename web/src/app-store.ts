import { createStore } from '@tanstack/react-store'
import type { RuleGroupType } from 'react-querybuilder'

import {
  createAPIKey as createAPIKeyRequest,
  createAuthSession,
  createAuthUser,
  createMeter as createMeterRequest,
  deleteAPIKey as deleteAPIKeyRequest,
  deleteAuthSession,
  deleteMeter as deleteMeterRequest,
  getSystemStats,
  listAPIKeys,
  listIngestions,
  listMeterStats,
  listMeters,
  listSubjects,
  listUsageBreakdowns,
  listUsageBuckets,
  listUsageDimensionValues,
  refreshAuthSession,
  updateMeter as updateMeterRequest,
  type APIKey,
  type APIKeyCreateResponse,
  type AuthSession,
  type Meter,
  type MeterCreateRequest,
  type MeterStats,
  type MeterUpdateRequest,
  type UsageBucket,
  type UsageBreakdown,
  type UsageDimensionValue,
  type IngestionRun,
  type SubjectStats,
  type SystemStats,
} from './api'
import {
  defaultFilterQuery,
  firstEqualRuleValue,
  metadataTypesByField,
  queryWithBreakdownFilter,
  queryWithAvailableMeter,
  selectedMeterSchemaKeys,
  usageFilterFromQuery,
  usageScopeFromQuery,
  usageTimeRangeFromQuery,
} from './lib/usage-query'
import type { LoadState } from './types'

export type MeterDimensionDraft = {
  id: string
  name: string
  type: string
}

type AppState = {
  auth: {
    checked: boolean
    loading: boolean
    loginError: string
    registerError: string
    session: AuthSession | null
  }
  apiKeys: {
    createdKey: APIKeyCreateResponse | null
    deleting: APIKey | null
    error: string
    items: APIKey[]
    saving: boolean
    status: LoadState
  }
  meters: {
    createDimensions: MeterDimensionDraft[]
    deleting: Meter | null
    editDimensions: MeterDimensionDraft[]
    editing: Meter | null
    error: string
    items: Meter[]
    saving: boolean
    stats: Record<string, MeterStats>
    status: LoadState
  }
  overview: {
    error: string
    ingestions: IngestionRun[]
    stats: SystemStats | null
    status: LoadState
    subjects: SubjectStats[]
  }
  usage: {
    breakdownError: string
    breakdowns: Record<string, UsageBreakdown[]>
    breakdownStatus: LoadState
    buckets: UsageBucket[]
    dimensionValues: Record<string, UsageDimensionValue[]>
    error: string
    filterQuery: RuleGroupType
    groupBy: string[]
    meters: Meter[]
    status: LoadState
  }
}

let meterDimensionID = 0
const domainSubjectField = 'subject'

export const appStore = createStore<AppState>({
  auth: {
    checked: false,
    loading: false,
    loginError: '',
    registerError: '',
    session: null,
  },
  apiKeys: {
    createdKey: null,
    deleting: null,
    error: '',
    items: [],
    saving: false,
    status: 'idle',
  },
  meters: {
    createDimensions: [newMeterDimensionDraft()],
    deleting: null,
    editDimensions: [],
    editing: null,
    error: '',
    items: [],
    saving: false,
    stats: {},
    status: 'idle',
  },
  overview: {
    error: '',
    ingestions: [],
    stats: null,
    status: 'idle',
    subjects: [],
  },
  usage: {
    breakdownError: '',
    breakdowns: {},
    breakdownStatus: 'idle',
    buckets: [],
    dimensionValues: {},
    error: '',
    filterQuery: defaultFilterQuery(),
    groupBy: [],
    meters: [],
    status: 'idle',
  },
})

export const appStoreActions = {
  clearCreatedAPIKey() {
    setAPIKeysState({ createdKey: null })
  },
  async createAPIKey(input: { name: string }) {
    setAPIKeysState({ createdKey: null, error: '', saving: true })
    try {
      const createdKey = await createAPIKeyRequest(input)
      setAPIKeysState({ createdKey })
      await appStoreActions.loadAPIKeys()
      return createdKey
    } catch (err) {
      setAPIKeysState({ error: errorMessage(err, 'Unable to create API key') })
      throw err
    } finally {
      setAPIKeysState({ saving: false })
    }
  },
  async createMeter(input: MeterCreateRequest) {
    setMetersState({ error: '', saving: true })
    try {
      await createMeterRequest(input)
      await appStoreActions.loadMeters()
    } catch (err) {
      setMetersState({ error: errorMessage(err, 'Unable to create meter') })
      throw err
    } finally {
      setMetersState({ saving: false })
    }
  },
  async deleteSelectedMeter() {
    const deleting = appStore.state.meters.deleting
    if (!deleting) {
      return
    }

    setMetersState({ error: '', saving: true })
    try {
      await deleteMeterRequest(deleting.id)
      setMetersState({ deleting: null })
      await appStoreActions.loadMeters()
    } catch (err) {
      setMetersState({ error: errorMessage(err, 'Unable to delete meter') })
      throw err
    } finally {
      setMetersState({ saving: false })
    }
  },
  async deleteSelectedAPIKey() {
    const deleting = appStore.state.apiKeys.deleting
    if (!deleting) {
      return
    }

    setAPIKeysState({ error: '', saving: true })
    try {
      await deleteAPIKeyRequest(deleting.id)
      setAPIKeysState((state) => ({
        createdKey: state.createdKey?.id === deleting.id ? null : state.createdKey,
        deleting: null,
        items: state.items.filter((item) => item.id !== deleting.id),
      }))
    } catch (err) {
      setAPIKeysState({ error: errorMessage(err, 'Unable to delete API key') })
      throw err
    } finally {
      setAPIKeysState({ saving: false })
    }
  },
  async ensureAuthUser() {
    const auth = appStore.state.auth
    if (auth.checked) {
      return auth.session?.user ?? null
    }

    setAuthState({ loading: true, loginError: '' })
    try {
      const session = await refreshAuthSession()
      setAuthState({ checked: true, loading: false, session })
      return session?.user ?? null
    } catch {
      setAuthState({ checked: true, loading: false, session: null })
      return null
    }
  },
  async loadAPIKeys() {
    setAPIKeysState({ error: '', status: 'loading' })
    try {
      const keys = await listAPIKeys()
      setAPIKeysState({ items: keys.items, status: 'ready' })
    } catch (err) {
      setAPIKeysState({ error: errorMessage(err, 'Unable to load API keys'), status: 'error' })
    }
  },
  async loadMeters() {
    setMetersState({ error: '', status: 'loading' })
    try {
      const [nextMeters, nextStats] = await Promise.all([listMeters(), listMeterStats()])
      setMetersState({
        items: nextMeters.items,
        stats: Object.fromEntries(nextStats.items.map((item) => [item.meter, item])),
        status: 'ready',
      })
    } catch (err) {
      setMetersState({ error: errorMessage(err, 'Unable to load meters'), status: 'error' })
    }
  },
  async loadOverview() {
    setOverviewState({ error: '', status: 'loading' })
    try {
      const [nextStats, nextSubjects, nextIngestions] = await Promise.all([
        getSystemStats(),
        listSubjects(),
        listIngestions(),
      ])
      setOverviewState({
        ingestions: nextIngestions.items,
        stats: nextStats,
        status: 'ready',
        subjects: nextSubjects.items,
      })
    } catch (err) {
      setOverviewState({ error: errorMessage(err, 'Unable to load overview'), status: 'error' })
    }
  },
  async loadUsageControls() {
    setUsageState({ error: '', status: 'loading' })
    try {
      const nextMeters = await listMeters()
      setUsageState((state) => ({
        meters: nextMeters.items,
        filterQuery: queryWithAvailableMeter(state.filterQuery, nextMeters.items),
        status: 'ready',
      }))
    } catch (err) {
      setUsageState({ error: errorMessage(err, 'Unable to load usage controls'), status: 'error' })
    }
  },
  async loadUsageDimensionValues() {
    const query = appStore.state.usage.filterQuery
    const meters = appStore.state.usage.meters
    const meter = firstEqualRuleValue(query, 'meter')
    const fields = selectedMeterSchemaKeys(meters, meter)
    if (!meter || fields.length === 0) {
      setUsageState({ dimensionValues: {} })
      return
    }

    const subject = firstEqualRuleValue(query, 'subject')
    let from = ''
    let to = ''
    try {
      const timeRange = usageTimeRangeFromQuery(query)
      from = timeRange.from
      to = timeRange.to
    } catch {
      // Discovery is still useful without a complete time window.
    }

    try {
      const values = await Promise.all(fields.map(async (field) => {
        const response = await listUsageDimensionValues({
          field,
          from,
          limit: 20,
          meter,
          subject,
          to,
        })
        return [field, response.items] as const
      }))
      setUsageState({ dimensionValues: Object.fromEntries(values) })
    } catch {
      setUsageState({ dimensionValues: {} })
    }
  },
  async loadUsageBreakdowns() {
    const query = appStore.state.usage.filterQuery
    const meters = appStore.state.usage.meters

    let scope: ReturnType<typeof usageScopeFromQuery>
    let timeRange: ReturnType<typeof usageTimeRangeFromQuery>
    try {
      scope = usageScopeFromQuery(query)
      timeRange = usageTimeRangeFromQuery(query)
    } catch {
      setUsageState({ breakdownError: '', breakdowns: {}, breakdownStatus: 'idle' })
      return
    }

    const fields = [domainSubjectField, ...selectedMeterSchemaKeys(meters, scope.meter)].slice(0, 5)
    if (fields.length === 0) {
      setUsageState({ breakdownError: '', breakdowns: {}, breakdownStatus: 'idle' })
      return
    }

    setUsageState({ breakdownError: '', breakdownStatus: 'loading' })
    try {
      const filter = usageFilterFromQuery(query, metadataTypesByField(meters, scope.meter))
      const breakdowns = await Promise.all(fields.map(async (field) => {
        const response = await listUsageBreakdowns({
          field,
          filter,
          from: timeRange.from,
          limit: 5,
          meter: scope.meter,
          subject: scope.subject || undefined,
          to: timeRange.to,
        })
        return [field, response.items] as const
      }))
      setUsageState({ breakdowns: Object.fromEntries(breakdowns), breakdownStatus: 'ready' })
    } catch (err) {
      setUsageState({
        breakdownError: errorMessage(err, 'Unable to load usage breakdowns'),
        breakdowns: {},
        breakdownStatus: 'error',
      })
    }
  },
  async login(input: { email: string; password: string }) {
    setAuthState({ loading: true, loginError: '', registerError: '' })
    try {
      const session = await createAuthSession(input)
      setAuthState({ checked: true, loading: false, session })
      return session
    } catch (err) {
      setAuthState({
        checked: true,
        loading: false,
        loginError: errorMessage(err, 'Unable to authenticate'),
        session: null,
      })
      throw err
    }
  },
  async logout() {
    setAuthState({ loading: true })
    try {
      await deleteAuthSession()
    } finally {
      setAuthState({ checked: true, loading: false, loginError: '', registerError: '', session: null })
    }
  },
  async register(input: { email: string; password: string }) {
    setAuthState({ loading: true, loginError: '', registerError: '' })
    try {
      await createAuthUser(input)
      const session = await createAuthSession(input)
      setAuthState({ checked: true, loading: false, session })
      return session
    } catch (err) {
      setAuthState({
        checked: true,
        loading: false,
        registerError: errorMessage(err, 'Unable to register'),
        session: null,
      })
      throw err
    }
  },
  resetUsageQuery() {
    const meters = appStore.state.usage.meters
    setUsageState({
      filterQuery: queryWithAvailableMeter(defaultFilterQuery(), meters),
      groupBy: [],
    })
  },
  addMeterCreateDimension() {
    setMetersState((state) => ({
      createDimensions: [...state.createDimensions, newMeterDimensionDraft()],
    }))
  },
  addMeterEditDimension() {
    setMetersState((state) => ({
      editDimensions: [...state.editDimensions, newMeterDimensionDraft()],
    }))
  },
  removeMeterCreateDimension(id: string) {
    setMetersState((state) => {
      const next = state.createDimensions.filter((row) => row.id !== id)
      return { createDimensions: next.length > 0 ? next : [newMeterDimensionDraft()] }
    })
  },
  removeMeterEditDimension(id: string) {
    setMetersState((state) => {
      const next = state.editDimensions.filter((row) => row.id !== id)
      return { editDimensions: next.length > 0 ? next : [newMeterDimensionDraft()] }
    })
  },
  resetMeterCreateDimensions() {
    setMetersState({ createDimensions: [newMeterDimensionDraft()] })
  },
  setMetersError(error: string) {
    setMetersState({ error })
  },
  setMeterDeleting(deleting: Meter | null) {
    setMetersState({ deleting })
  },
  setAPIKeyDeleting(deleting: APIKey | null) {
    setAPIKeysState({ deleting })
  },
  setMeterEditing(editing: Meter | null) {
    setMetersState({
      editing,
      editDimensions: editing ? meterDimensionDraftsFromSchema(editing.metadata_schema) : [],
    })
  },
  updateMeterCreateDimension(id: string, update: Partial<Omit<MeterDimensionDraft, 'id'>>) {
    setMetersState((state) => ({
      createDimensions: state.createDimensions.map((row) => row.id === id ? { ...row, ...update } : row),
    }))
  },
  updateMeterEditDimension(id: string, update: Partial<Omit<MeterDimensionDraft, 'id'>>) {
    setMetersState((state) => ({
      editDimensions: state.editDimensions.map((row) => row.id === id ? { ...row, ...update } : row),
    }))
  },
  setUsageFilterQuery(filterQuery: RuleGroupType) {
    setUsageState({ filterQuery })
  },
  applyUsageBreakdownFilter(field: string, value: string) {
    setUsageState((state) => ({
      filterQuery: queryWithBreakdownFilter(state.filterQuery, field, value),
    }))
  },
  setUsageGroupBy(groupBy: string[]) {
    setUsageState({ groupBy })
  },
  toggleUsageGroupBy(field: string) {
    setUsageState((state) => {
      const groupBy = state.groupBy.includes(field)
        ? state.groupBy.filter((item) => item !== field)
        : [...state.groupBy, field]
      return { groupBy }
    })
  },
  async submitUsageQuery(groupByValue: string[], limit = 500, bucketSize = 'day') {
    setUsageState({ error: '', status: 'loading' })
    try {
      const query = appStore.state.usage.filterQuery
      const scope = usageScopeFromQuery(query)
      const timeRange = usageTimeRangeFromQuery(query)
      const filter = usageFilterFromQuery(query, metadataTypesByField(appStore.state.usage.meters, scope.meter))
      const groupBy = groupByValue.filter(Boolean)
      const buckets = await listUsageBuckets({
        bucket_size: bucketSize,
        filter,
        from: timeRange.from,
        group_by: groupBy.length > 0 ? groupBy : undefined,
        limit,
        meter: scope.meter,
        subject: scope.subject || undefined,
        to: timeRange.to,
      })
      setUsageState({ buckets, status: 'ready' })
    } catch (err) {
      setUsageState({ error: errorMessage(err, 'Unable to query usage'), status: 'error' })
    }
  },
  async updateEditingMeter(input: MeterUpdateRequest) {
    const editing = appStore.state.meters.editing
    if (!editing) {
      return
    }

    setMetersState({ error: '', saving: true })
    try {
      await updateMeterRequest(editing.id, input)
      setMetersState({ editing: null })
      await appStoreActions.loadMeters()
    } catch (err) {
      setMetersState({ error: errorMessage(err, 'Unable to update meter') })
      throw err
    } finally {
      setMetersState({ saving: false })
    }
  },
}

function errorMessage(err: unknown, fallback: string) {
  return err instanceof Error ? err.message : fallback
}

function setAuthState(update: Partial<AppState['auth']>) {
  appStore.setState((state) => ({
    ...state,
    auth: {
      ...state.auth,
      ...update,
    },
  }))
}

function setAPIKeysState(update: Partial<AppState['apiKeys']> | ((state: AppState['apiKeys']) => Partial<AppState['apiKeys']>)) {
  appStore.setState((state) => ({
    ...state,
    apiKeys: {
      ...state.apiKeys,
      ...(typeof update === 'function' ? update(state.apiKeys) : update),
    },
  }))
}

function setMetersState(update: Partial<AppState['meters']> | ((state: AppState['meters']) => Partial<AppState['meters']>)) {
  appStore.setState((state) => ({
    ...state,
    meters: {
      ...state.meters,
      ...(typeof update === 'function' ? update(state.meters) : update),
    },
  }))
}

function setOverviewState(update: Partial<AppState['overview']>) {
  appStore.setState((state) => ({
    ...state,
    overview: {
      ...state.overview,
      ...update,
    },
  }))
}

function setUsageState(update: Partial<AppState['usage']> | ((state: AppState['usage']) => Partial<AppState['usage']>)) {
  appStore.setState((state) => ({
    ...state,
    usage: {
      ...state.usage,
      ...(typeof update === 'function' ? update(state.usage) : update),
    },
  }))
}

function newMeterDimensionDraft(name = '', type = 'string'): MeterDimensionDraft {
  meterDimensionID += 1
  return {
    id: `meter-dimension-${meterDimensionID}`,
    name,
    type,
  }
}

function meterDimensionDraftsFromSchema(schema: Record<string, string>) {
  const rows = Object.entries(schema || {})
    .sort(([left], [right]) => left.localeCompare(right))
    .map(([name, type]) => newMeterDimensionDraft(name, type))
  return rows.length > 0 ? rows : [newMeterDimensionDraft()]
}
