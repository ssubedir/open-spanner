import { createStore } from '@tanstack/react-store'
import type { RuleGroupType } from 'react-querybuilder'

import {
  createAPIKey as createAPIKeyRequest,
  createAuthSession,
  createAuthUser,
  createMeter as createMeterRequest,
  createSavedUsageQuery,
  deleteAPIKey as deleteAPIKeyRequest,
  deleteAuthSession,
  deleteMeter as deleteMeterRequest,
  deleteSavedUsageQuery,
  exportUsageBuckets,
  exportUsageEvents,
  getSystemStats,
  listAPIKeys,
  listIngestions,
  listMeterStats,
  listMeters,
  listSavedUsageQueries,
  listSubjectEvents,
  listSubjects,
  listUsageBreakdowns,
  listUsageBuckets,
  listUsageDimensionValues,
  refreshAuthSession,
  updateMeter as updateMeterRequest,
  updateSavedUsageQuery,
  type APIKey,
  type APIKeyCreateResponse,
  type AuthSession,
  type Meter,
  type MeterDimension,
  type MeterCreateRequest,
  type MeterStats,
  type MeterUpdateRequest,
  type SavedUsageQuery,
  type UsageBucket,
  type UsageBreakdown,
  type UsageDimensionValue,
  type UsageEvent,
  type IngestionRun,
  type SubjectStats,
  type SystemStats,
} from './api'
import {
  defaultFilterQuery,
  firstEqualRuleValue,
  metadataEqualsFromQuery,
  metadataTypesByField,
  queryFromSavedValue,
  queryWithBreakdownFilter,
  queryWithAvailableMeter,
  queryWithSubject,
  selectedMeterSchemaKeys,
  unsupportedBucketExportRuleCount,
  usageFilterFromQuery,
  usageScopeFromQuery,
  usageTimeRangeFromQuery,
} from './lib/usage-query'
import { downloadBlob, safeDownloadName } from './lib/download'
import type { LoadState } from './types'

export type MeterDimensionDraft = {
  description: string
  displayName: string
  id: string
  name: string
  required: boolean
  type: string
}

export type PinnedUsageQuerySummary = {
  bucketSize: string
  error: string
  lastBucket: string
  query: SavedUsageQuery
  rows: number
  total: number
  unit: string
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
    pinnedUsageQueries: PinnedUsageQuerySummary[]
    stats: SystemStats | null
    status: LoadState
    subjects: SubjectStats[]
  }
  subjects: {
    detailStatus: LoadState
    error: string
    events: UsageEvent[]
    exportError: string
    exporting: boolean
    items: SubjectStats[]
    searchQuery: string
    selectedSubject: string
    status: LoadState
  }
  usage: {
    bucketSize: string
    breakdownError: string
    breakdowns: Record<string, UsageBreakdown[]>
    breakdownStatus: LoadState
    buckets: UsageBucket[]
    dimensionValues: Record<string, UsageDimensionValue[]>
    error: string
    exportError: string
    exporting: boolean
    filterQuery: RuleGroupType
    groupBy: string[]
    limit: number
    meters: Meter[]
    savedQueryDeleting: SavedUsageQuery | null
    savedQueryError: string
    savedQueryName: string
    savedQuerySaving: boolean
    savedQueryStatus: LoadState
    savedQueries: SavedUsageQuery[]
    selectedSavedQueryID: string
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
    pinnedUsageQueries: [],
    stats: null,
    status: 'idle',
    subjects: [],
  },
  subjects: {
    detailStatus: 'idle',
    error: '',
    events: [],
    exportError: '',
    exporting: false,
    items: [],
    searchQuery: '',
    selectedSubject: '',
    status: 'idle',
  },
  usage: {
    bucketSize: 'day',
    breakdownError: '',
    breakdowns: {},
    breakdownStatus: 'idle',
    buckets: [],
    dimensionValues: {},
    error: '',
    exportError: '',
    exporting: false,
    filterQuery: defaultFilterQuery(),
    groupBy: [],
    limit: 500,
    meters: [],
    savedQueryDeleting: null,
    savedQueryError: '',
    savedQueryName: '',
    savedQuerySaving: false,
    savedQueryStatus: 'idle',
    savedQueries: [],
    selectedSavedQueryID: '',
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
      const [nextStats, nextSubjects, nextIngestions, savedQueries, meters] = await Promise.all([
        getSystemStats(),
        listSubjects(),
        listIngestions(),
        listSavedUsageQueries(),
        listMeters(),
      ])
      const pinned = savedQueries.items
        .filter((query) => query.pinned)
        .sort((left, right) => left.position - right.position || left.name.localeCompare(right.name))
        .slice(0, 6)
      const pinnedUsageQueries = await Promise.all(pinned.map((query) => summarizePinnedUsageQuery(query, meters.items)))
      setOverviewState({
        ingestions: nextIngestions.items,
        pinnedUsageQueries,
        stats: nextStats,
        status: 'ready',
        subjects: nextSubjects.items,
      })
    } catch (err) {
      setOverviewState({ error: errorMessage(err, 'Unable to load overview'), status: 'error' })
    }
  },
  async loadSubjects(preferredSubject = '') {
    setSubjectsState({ error: '', status: 'loading' })
    try {
      const subjects = await listSubjects(50)
      const selectedSubject = preferredSubject.trim() || selectedSubjectForList(appStore.state.subjects.selectedSubject, subjects.items)
      setSubjectsState({
        items: subjects.items,
        selectedSubject,
        status: 'ready',
      })
      if (selectedSubject) {
        await appStoreActions.loadSubjectEvents(selectedSubject)
      } else {
        setSubjectsState({ detailStatus: 'idle', events: [] })
      }
    } catch (err) {
      setSubjectsState({ error: errorMessage(err, 'Unable to load subjects'), status: 'error' })
    }
  },
  async loadSubjectEvents(subject: string) {
    if (!subject) {
      setSubjectsState({ detailStatus: 'idle', events: [], selectedSubject: '' })
      return
    }

    setSubjectsState({ detailStatus: 'loading', error: '', exportError: '', selectedSubject: subject })
    try {
      const events = await listSubjectEvents(subject, 25)
      setSubjectsState({ detailStatus: 'ready', events })
    } catch (err) {
      setSubjectsState({ detailStatus: 'error', error: errorMessage(err, 'Unable to load subject activity'), events: [] })
    }
  },
  async exportSelectedSubjectEvents() {
    const subject = appStore.state.subjects.selectedSubject
    if (!subject) {
      return
    }

    setSubjectsState({ exportError: '', exporting: true })
    try {
      const blob = await exportUsageEvents({ limit: 1000, subject })
      downloadBlob(blob, `${safeDownloadName(subject)}-usage-events.csv`)
    } catch (err) {
      setSubjectsState({ exportError: errorMessage(err, 'Unable to export subject events') })
    } finally {
      setSubjectsState({ exporting: false })
    }
  },
  async loadUsageControls() {
    setUsageState({ error: '', exportError: '', savedQueryError: '', savedQueryStatus: 'loading', status: 'loading' })
    try {
      const [nextMeters, savedQueries] = await Promise.all([
        listMeters(),
        listSavedUsageQueries(),
      ])
      setUsageState((state) => ({
        meters: nextMeters.items,
        savedQueries: savedQueries.items,
        savedQueryStatus: 'ready',
        filterQuery: queryWithAvailableMeter(state.filterQuery, nextMeters.items),
        status: 'ready',
      }))
    } catch (err) {
      setUsageState({
        error: errorMessage(err, 'Unable to load usage controls'),
        savedQueryError: errorMessage(err, 'Unable to load saved queries'),
        savedQueryStatus: 'error',
        status: 'error',
      })
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
      bucketSize: 'day',
      exportError: '',
      filterQuery: queryWithAvailableMeter(defaultFilterQuery(), meters),
      groupBy: [],
      limit: 500,
      savedQueryName: '',
      selectedSavedQueryID: '',
    })
  },
  prepareUsageForSubject(subject: string) {
    const normalizedSubject = subject.trim()
    if (!normalizedSubject) {
      return
    }

    setUsageState((state) => ({
      buckets: [],
      error: '',
      exportError: '',
      filterQuery: queryWithSubject(state.filterQuery, normalizedSubject),
      savedQueryName: '',
      selectedSavedQueryID: '',
      status: 'idle',
    }))
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
  setSubjectSearchQuery(searchQuery: string) {
    setSubjectsState({ searchQuery })
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
      editDimensions: editing ? meterDimensionDraftsFromMeter(editing) : [],
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
  setUsageBucketSize(bucketSize: string) {
    setUsageState({ bucketSize })
  },
  setUsageLimit(limit: number) {
    setUsageState({ limit })
  },
  applyUsageBreakdownFilter(field: string, value: string) {
    setUsageState((state) => ({
      filterQuery: queryWithBreakdownFilter(state.filterQuery, field, value),
    }))
  },
  setSavedUsageQueryDeleting(deleting: SavedUsageQuery | null) {
    setUsageState({ savedQueryDeleting: deleting })
  },
  setSavedUsageQueryName(name: string) {
    setUsageState({ savedQueryName: name })
  },
  selectSavedUsageQuery(id: string) {
    if (!id) {
      setUsageState({ savedQueryName: '', selectedSavedQueryID: '' })
      return
    }

    setUsageState((state) => {
      const saved = state.savedQueries.find((item) => item.id === id)
      if (!saved) {
        return { selectedSavedQueryID: '' }
      }

      return usageStateFromSavedQuery(saved, state)
    })
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
    setUsageState({ error: '', exportError: '', status: 'loading' })
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
  async exportCurrentUsageBuckets(groupByValue: string[], limit = 500, bucketSize = 'day') {
    setUsageState({ exportError: '', exporting: true })
    try {
      const query = appStore.state.usage.filterQuery
      const unsupportedRules = unsupportedBucketExportRuleCount(query)
      if (unsupportedRules > 0) {
        throw new Error('CSV export supports meter, subject, timestamp range, group-by, and equals metadata filters. Remove advanced filter rules before exporting.')
      }

      const scope = usageScopeFromQuery(query)
      const timeRange = usageTimeRangeFromQuery(query)
      const groupBy = groupByValue.filter(Boolean)
      const blob = await exportUsageBuckets({
        bucket_size: bucketSize,
        from: timeRange.from,
        group_by: groupBy.length > 0 ? groupBy : undefined,
        limit,
        metadata: metadataEqualsFromQuery(query),
        meter: scope.meter,
        subject: scope.subject || undefined,
        to: timeRange.to,
      })
      const fileParts = ['usage-buckets', scope.meter, scope.subject].filter((part): part is string => Boolean(part))
      downloadBlob(blob, `${fileParts.map(safeDownloadName).join('-')}.csv`)
    } catch (err) {
      setUsageState({ exportError: errorMessage(err, 'Unable to export usage buckets') })
    } finally {
      setUsageState({ exporting: false })
    }
  },
  async saveCurrentUsageQuery() {
    const state = appStore.state.usage
    const selectedID = state.selectedSavedQueryID
    setUsageState({ savedQueryError: '', savedQuerySaving: true })
    try {
      const input = {
        bucket_size: state.bucketSize,
        group_by: state.groupBy,
        limit: state.limit,
        name: state.savedQueryName,
        pinned: state.savedQueries.find((item) => item.id === selectedID)?.pinned ?? false,
        position: state.savedQueries.find((item) => item.id === selectedID)?.position ?? 0,
        query: state.filterQuery,
      }
      const saved = selectedID
        ? await updateSavedUsageQuery(selectedID, input)
        : await createSavedUsageQuery(input)
      const list = await listSavedUsageQueries()
      setUsageState({
        savedQueries: list.items,
        savedQueryName: saved.name,
        savedQueryStatus: 'ready',
        selectedSavedQueryID: saved.id,
      })
      return saved
    } catch (err) {
      setUsageState({ savedQueryError: errorMessage(err, 'Unable to save usage query') })
      throw err
    } finally {
      setUsageState({ savedQuerySaving: false })
    }
  },
  applySavedUsageQuery(query: SavedUsageQuery) {
    setUsageState((state) => ({
      ...usageStateFromSavedQuery(query, state),
      savedQueries: mergeSavedUsageQuery(state.savedQueries, query),
    }))
  },
  async deleteSelectedSavedUsageQuery() {
    const deleting = appStore.state.usage.savedQueryDeleting
    if (!deleting) {
      return
    }

    setUsageState({ savedQueryError: '', savedQuerySaving: true })
    try {
      await deleteSavedUsageQuery(deleting.id)
      setUsageState((state) => ({
        savedQueries: state.savedQueries.filter((item) => item.id !== deleting.id),
        savedQueryDeleting: null,
        savedQueryName: state.selectedSavedQueryID === deleting.id ? '' : state.savedQueryName,
        selectedSavedQueryID: state.selectedSavedQueryID === deleting.id ? '' : state.selectedSavedQueryID,
      }))
    } catch (err) {
      setUsageState({ savedQueryError: errorMessage(err, 'Unable to delete usage query') })
      throw err
    } finally {
      setUsageState({ savedQuerySaving: false })
    }
  },
  async toggleSavedUsageQueryPinned(query: SavedUsageQuery) {
    const state = appStore.state.usage
    const pinned = !query.pinned
    const position = pinned
      ? nextPinnedPosition(state.savedQueries, query.id)
      : 0

    setUsageState({ savedQueryError: '', savedQuerySaving: true })
    try {
      const updated = await updateSavedUsageQuery(query.id, savedUsageQueryRequest(query, { pinned, position }))
      const list = await listSavedUsageQueries()
      setUsageState({
        savedQueries: list.items,
        selectedSavedQueryID: state.selectedSavedQueryID === query.id ? updated.id : state.selectedSavedQueryID,
      })
      return updated
    } catch (err) {
      setUsageState({ savedQueryError: errorMessage(err, 'Unable to update pinned query') })
      throw err
    } finally {
      setUsageState({ savedQuerySaving: false })
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

function setSubjectsState(update: Partial<AppState['subjects']>) {
  appStore.setState((state) => ({
    ...state,
    subjects: {
      ...state.subjects,
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

function selectedSubjectForList(selectedSubject: string, subjects: SubjectStats[]) {
  if (selectedSubject && subjects.some((subject) => subject.subject === selectedSubject)) {
    return selectedSubject
  }
  return subjects[0]?.subject ?? ''
}

async function summarizePinnedUsageQuery(query: SavedUsageQuery, meters: Meter[]): Promise<PinnedUsageQuerySummary> {
  try {
    const filterQuery = queryFromSavedValue(query.query, defaultFilterQuery())
    const scope = usageScopeFromQuery(filterQuery)
    const timeRange = usageTimeRangeFromQuery(filterQuery)
    const filter = usageFilterFromQuery(filterQuery, metadataTypesByField(meters, scope.meter))
    const buckets = await listUsageBuckets({
      bucket_size: query.bucket_size || 'day',
      filter,
      from: timeRange.from,
      group_by: query.group_by && query.group_by.length > 0 ? query.group_by : undefined,
      limit: query.limit || 500,
      meter: scope.meter,
      subject: scope.subject || undefined,
      to: timeRange.to,
    })

    return {
      bucketSize: query.bucket_size || 'day',
      error: '',
      lastBucket: buckets.reduce((latest, bucket) => bucket.bucket_start > latest ? bucket.bucket_start : latest, ''),
      query,
      rows: buckets.length,
      total: buckets.reduce((sum, bucket) => sum + bucket.quantity, 0),
      unit: buckets.find((bucket) => bucket.unit)?.unit || '',
    }
  } catch (err) {
    return {
      bucketSize: query.bucket_size || 'day',
      error: errorMessage(err, 'Unable to load pinned query'),
      lastBucket: '',
      query,
      rows: 0,
      total: 0,
      unit: '',
    }
  }
}

function usageStateFromSavedQuery(query: SavedUsageQuery, state: AppState['usage']): Partial<AppState['usage']> {
  return {
    bucketSize: query.bucket_size || 'day',
    filterQuery: queryFromSavedValue(query.query, state.filterQuery),
    groupBy: query.group_by || [],
    limit: query.limit || 500,
    savedQueryName: query.name,
    selectedSavedQueryID: query.id,
  }
}

function mergeSavedUsageQuery(items: SavedUsageQuery[], query: SavedUsageQuery) {
  const next = items.filter((item) => item.id !== query.id)
  next.push(query)
  return next.sort((left, right) => Number(right.pinned) - Number(left.pinned) || left.position - right.position || left.name.localeCompare(right.name))
}

function savedUsageQueryRequest(query: SavedUsageQuery, overrides: Partial<Pick<SavedUsageQuery, 'pinned' | 'position'>> = {}) {
  return {
    bucket_size: query.bucket_size,
    group_by: query.group_by,
    limit: query.limit,
    name: query.name,
    pinned: overrides.pinned ?? query.pinned,
    position: overrides.position ?? query.position,
    query: query.query,
  }
}

function nextPinnedPosition(items: SavedUsageQuery[], excludeID: string) {
  return items
    .filter((query) => query.pinned && query.id !== excludeID)
    .reduce((position, query) => Math.max(position, query.position), 0) + 1
}

function newMeterDimensionDraft(name = '', type = 'string', displayName = '', description = '', required = true): MeterDimensionDraft {
  meterDimensionID += 1
  return {
    description,
    displayName,
    id: `meter-dimension-${meterDimensionID}`,
    name,
    required,
    type,
  }
}

function meterDimensionDraftsFromMeter(meter: Meter) {
  const dimensions = normalizedMeterDimensions(meter)
  if (dimensions.length > 0) {
    return dimensions.map((dimension) => newMeterDimensionDraft(
      dimension.name,
      dimension.type,
      dimension.display_name,
      dimension.description,
      dimension.required,
    ))
  }
  return meterDimensionDraftsFromSchema(meter.metadata_schema)
}

function meterDimensionDraftsFromSchema(schema: Record<string, string>) {
  const rows = Object.entries(schema || {})
    .sort(([left], [right]) => left.localeCompare(right))
    .map(([name, type]) => newMeterDimensionDraft(name, type))
  return rows.length > 0 ? rows : [newMeterDimensionDraft()]
}

function normalizedMeterDimensions(meter: Meter): MeterDimension[] {
  if (meter.dimensions && meter.dimensions.length > 0) {
    return meter.dimensions
  }
  return Object.entries(meter.metadata_schema || {})
    .sort(([left], [right]) => left.localeCompare(right))
    .map(([name, type]) => ({
      description: '',
      display_name: humanizeDimensionName(name),
      name,
      required: true,
      type,
    }))
}

function humanizeDimensionName(name: string) {
  return name
    .replace(/^metadata\./, '')
    .split(/[._-]/)
    .filter(Boolean)
    .map((part) => part.charAt(0).toUpperCase() + part.slice(1))
    .join(' ')
}
