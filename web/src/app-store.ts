import { createStore } from '@tanstack/react-store'
import type { RuleGroupType } from 'react-querybuilder'

import {
  createAuthSession,
  createAuthUser,
  createMeter as createMeterRequest,
  createUsage as createUsageRequest,
  deleteAuthSession,
  deleteMeter as deleteMeterRequest,
  getSystemStats,
  listIngestions,
  listMeterStats,
  listMeters,
  listSubjects,
  listUsageBuckets,
  refreshAuthSession,
  updateMeter as updateMeterRequest,
  type AuthSession,
  type Meter,
  type MeterCreateRequest,
  type MeterStats,
  type MeterUpdateRequest,
  type UsageBucket,
  type UsageCreateRequest,
  type IngestionRun,
  type SubjectStats,
  type SystemStats,
} from './api'
import {
  defaultFilterQuery,
  queryWithAvailableMeter,
  usageFilterFromQuery,
  usageScopeFromQuery,
  usageTimeRangeFromQuery,
} from './lib/usage-query'
import type { LoadState } from './types'

type AppState = {
  auth: {
    checked: boolean
    loading: boolean
    loginError: string
    registerError: string
    session: AuthSession | null
  }
  meters: {
    deleting: Meter | null
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
    buckets: UsageBucket[]
    createOpen: boolean
    error: string
    filterQuery: RuleGroupType
    groupBy: string
    meters: Meter[]
    saving: boolean
    status: LoadState
  }
}

export const appStore = createStore<AppState>({
  auth: {
    checked: false,
    loading: false,
    loginError: '',
    registerError: '',
    session: null,
  },
  meters: {
    deleting: null,
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
    buckets: [],
    createOpen: false,
    error: '',
    filterQuery: defaultFilterQuery(),
    groupBy: '',
    meters: [],
    saving: false,
    status: 'idle',
  },
})

export const appStoreActions = {
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
  async createUsage(input: UsageCreateRequest, groupByValue: string) {
    setUsageState({ error: '', saving: true })
    try {
      await createUsageRequest(input)
      setUsageState({ createOpen: false })
      await appStoreActions.submitUsageQuery(groupByValue)
    } catch (err) {
      setUsageState({ error: errorMessage(err, 'Unable to create usage') })
      throw err
    } finally {
      setUsageState({ saving: false })
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
        registerError: registrationErrorMessage(err),
        session: null,
      })
      throw err
    }
  },
  resetUsageQuery() {
    const meters = appStore.state.usage.meters
    setUsageState({
      filterQuery: queryWithAvailableMeter(defaultFilterQuery(), meters),
      groupBy: '',
    })
  },
  setMeterDeleting(deleting: Meter | null) {
    setMetersState({ deleting })
  },
  setMeterEditing(editing: Meter | null) {
    setMetersState({ editing })
  },
  setUsageCreateOpen(createOpen: boolean) {
    setUsageState({ createOpen })
  },
  setUsageFilterQuery(filterQuery: RuleGroupType) {
    setUsageState({ filterQuery })
  },
  setUsageGroupBy(groupBy: string) {
    setUsageState({ groupBy })
  },
  async submitUsageQuery(groupByValue: string, limit = 500, bucketSize = 'day') {
    setUsageState({ error: '', status: 'loading' })
    try {
      const query = appStore.state.usage.filterQuery
      const scope = usageScopeFromQuery(query)
      const timeRange = usageTimeRangeFromQuery(query)
      const filter = usageFilterFromQuery(query)
      const buckets = await listUsageBuckets({
        bucket_size: bucketSize,
        filter,
        from: timeRange.from,
        group_by: groupByValue || undefined,
        limit,
        meter: scope.meter,
        subject: scope.subject,
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

function registrationErrorMessage(err: unknown) {
  const message = errorMessage(err, 'Unable to register')
  if (message.includes('user creation requires authentication')) {
    return 'Registration is only available before an admin account exists'
  }
  return message
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

function setMetersState(update: Partial<AppState['meters']>) {
  appStore.setState((state) => ({
    ...state,
    meters: {
      ...state.meters,
      ...update,
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
