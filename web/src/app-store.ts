import { createStore } from '@tanstack/react-store'
import type { RuleGroupType } from 'react-querybuilder'

import {
  createAlertRule,
  APIError,
  cancelUsageExportJob,
  createAPIKey as createAPIKeyRequest,
  createAuthSession,
  createAuthUser,
  createMeter as createMeterRequest,
  createSavedUsageQuery,
  createUsageExportJob,
  deleteAlertRule,
  deleteAPIKey as deleteAPIKeyRequest,
  deleteAuthSession,
  deleteMeter as deleteMeterRequest,
  deleteSavedUsageQuery,
  downloadUsageExportJob,
  exportUsageBuckets,
  exportUsageEvents,
  evaluateAlertRule,
  getSystemStats,
  listAlertEvents,
  listAlertRules,
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
  listUsageEvents,
  listUsageExportJobs,
  refreshAuthSession,
  retryUsageExportJob,
  updateAlertRule,
  updateMeter as updateMeterRequest,
  updateSavedUsageQuery,
  type AlertEvent,
  type AlertRule,
  type AlertRuleRequest,
  type AlertRuleUpdateRequest,
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
  type UsageBucketExportQuery,
  type UsageBreakdown,
  type UsageDimensionValue,
  type UsageEvent,
  type UsageEventQuery,
  type UsageExportJob,
  type IngestionRun,
  type SubjectStats,
  type SystemStats,
} from './api'
import {
  defaultFilterQuery,
  firstEqualRuleValue,
  metadataTypesByField,
  queryFromSavedValue,
  queryWithBreakdownFilter,
  queryWithAvailableMeter,
  queryWithMeter,
  queryWithSubject,
  selectedMeterSchemaKeys,
  usageFilterFromQuery,
  usageScopeFromQuery,
  usageTimeRangeFromQuery,
} from './lib/usage-query'
import { downloadBlob, safeDownloadName } from './lib/download'
import type { LoadState } from './types'

type UsageExportKind = '' | 'buckets' | 'events' | 'job'

export type MeterDimensionDraft = {
  deprecated: boolean
  description: string
  displayName: string
  id: string
  name: string
  originalDeprecated?: boolean
  originalName?: string
  originalRequired?: boolean
  originalType?: string
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
  alerts: {
    deleting: AlertRule | null
    editing: AlertRule | null
    error: string
    events: AlertEvent[]
    eventStatus: LoadState
    items: AlertRule[]
    meters: Meter[]
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
    events: UsageEvent[]
    eventsError: string
    eventsStatus: LoadState
    exportError: string
    exportJobDownloading: string
    exportJobError: string
    exportJobLimit: number
    exportJobMutating: string
    exportJobStatus: LoadState
    exportJobs: UsageExportJob[]
    exporting: UsageExportKind
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
    selectedUsageEvent: UsageEvent | null
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
  alerts: {
    deleting: null,
    editing: null,
    error: '',
    events: [],
    eventStatus: 'idle',
    items: [],
    meters: [],
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
    events: [],
    eventsError: '',
    eventsStatus: 'idle',
    exportError: '',
    exportJobDownloading: '',
    exportJobError: '',
    exportJobLimit: 8,
    exportJobMutating: '',
    exportJobStatus: 'idle',
    exportJobs: [],
    exporting: '',
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
    selectedUsageEvent: null,
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
  async loadAlerts() {
    setAlertsState({ error: '', eventStatus: 'loading', status: 'loading' })
    try {
      const meters = await listMeters()
      setAlertsState({ meters: meters.items })

      const [rules, events] = await Promise.all([
        listAlertRules(),
        listAlertEvents(),
      ])
      setAlertsState({
        events: events.items,
        eventStatus: 'ready',
        items: rules.items,
        status: 'ready',
      })
    } catch (err) {
      setAlertsState({
        error: errorMessage(err, 'Unable to load alerts'),
        eventStatus: 'error',
        status: 'error',
      })
    }
  },
  async createAlert(input: AlertRuleRequest) {
    setAlertsState({ error: '', saving: true })
    try {
      await createAlertRule(input)
      await appStoreActions.loadAlerts()
    } catch (err) {
      setAlertsState({ error: errorMessage(err, 'Unable to create alert') })
      throw err
    } finally {
      setAlertsState({ saving: false })
    }
  },
  async updateEditingAlert(input: AlertRuleUpdateRequest) {
    const editing = appStore.state.alerts.editing
    if (!editing) {
      return
    }

    setAlertsState({ error: '', saving: true })
    try {
      await updateAlertRule(editing.id, input)
      setAlertsState({ editing: null })
      await appStoreActions.loadAlerts()
    } catch (err) {
      setAlertsState({ error: errorMessage(err, 'Unable to update alert') })
      throw err
    } finally {
      setAlertsState({ saving: false })
    }
  },
  async deleteSelectedAlert() {
    const deleting = appStore.state.alerts.deleting
    if (!deleting) {
      return
    }

    setAlertsState({ error: '', saving: true })
    try {
      await deleteAlertRule(deleting.id)
      setAlertsState((state) => ({
        deleting: null,
        events: state.events.filter((event) => event.rule_id !== deleting.id),
        items: state.items.filter((item) => item.id !== deleting.id),
      }))
    } catch (err) {
      setAlertsState({ error: errorMessage(err, 'Unable to delete alert') })
      throw err
    } finally {
      setAlertsState({ saving: false })
    }
  },
  async evaluateAlert(rule: AlertRule) {
    setAlertsState({ error: '', saving: true })
    try {
      const result = await evaluateAlertRule(rule.id)
      setAlertsState((state) => ({
        events: result.event ? [result.event, ...state.events.filter((event) => event.id !== result.event?.id)].slice(0, 25) : state.events,
        items: state.items.map((item) => item.id === rule.id ? result.rule : item),
      }))
    } catch (err) {
      setAlertsState({ error: errorMessage(err, 'Unable to evaluate alert') })
      throw err
    } finally {
      setAlertsState({ saving: false })
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
    setUsageState({
      error: '',
      exportError: '',
      exportJobError: '',
      exportJobStatus: 'loading',
      savedQueryError: '',
      savedQueryStatus: 'loading',
      status: 'loading',
    })
    try {
      const [nextMeters, savedQueries, exportJobs] = await Promise.all([
        listMeters(),
        listSavedUsageQueries(),
        listUsageExportJobs(),
      ])
      setUsageState((state) => ({
        exportJobLimit: 8,
        exportJobs: exportJobs.items,
        exportJobStatus: 'ready',
        meters: nextMeters.items,
        savedQueries: savedQueries.items,
        savedQueryStatus: 'ready',
        filterQuery: queryWithAvailableMeter(state.filterQuery, nextMeters.items),
        status: 'ready',
      }))
    } catch (err) {
      setUsageState({
        error: errorMessage(err, 'Unable to load usage controls'),
        exportJobError: errorMessage(err, 'Unable to load export jobs'),
        exportJobStatus: 'error',
        savedQueryError: errorMessage(err, 'Unable to load saved queries'),
        savedQueryStatus: 'error',
        status: 'error',
      })
    }
  },
  async loadUsageExportJobs(limit = appStore.state.usage.exportJobLimit || 8) {
    setUsageState({ exportJobError: '', exportJobStatus: 'loading' })
    try {
      const exportJobs = await listUsageExportJobs(limit)
      setUsageState({ exportJobLimit: limit, exportJobs: exportJobs.items, exportJobStatus: 'ready' })
    } catch (err) {
      setUsageState({
        exportJobError: errorMessage(err, 'Unable to load export jobs'),
        exportJobStatus: 'error',
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
        loginError: authErrorMessage(err, 'Unable to sign in'),
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
        registerError: registerErrorMessage(err, 'Unable to create account'),
        session: null,
      })
      throw err
    }
  },
  resetUsageQuery() {
    const meters = appStore.state.usage.meters
    setUsageState({
      bucketSize: 'day',
      events: [],
      eventsError: '',
      eventsStatus: 'idle',
      exportError: '',
      filterQuery: queryWithAvailableMeter(defaultFilterQuery(), meters),
      groupBy: [],
      limit: 500,
      savedQueryName: '',
      selectedSavedQueryID: '',
      selectedUsageEvent: null,
    })
  },
  prepareUsageForSubject(subject: string, meter = '') {
    const normalizedSubject = subject.trim()
    if (!normalizedSubject) {
      return
    }
    const normalizedMeter = meter.trim()

    setUsageState((state) => ({
      buckets: [],
      error: '',
      events: [],
      eventsError: '',
      eventsStatus: 'idle',
      exportError: '',
      filterQuery: normalizedMeter
        ? queryWithMeter(queryWithSubject(state.filterQuery, normalizedSubject), normalizedMeter)
        : queryWithSubject(state.filterQuery, normalizedSubject),
      savedQueryName: '',
      selectedSavedQueryID: '',
      selectedUsageEvent: null,
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
      editDimensions: [...state.editDimensions, newMeterDimensionDraft('', 'string', '', '', !meterHasUsage(state.editing, state.stats))],
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
      return { editDimensions: next.length > 0 ? next : [newMeterDimensionDraft('', 'string', '', '', !meterHasUsage(state.editing, state.stats))] }
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
  setAlertDeleting(deleting: AlertRule | null) {
    setAlertsState({ deleting })
  },
  setAlertEditing(editing: AlertRule | null) {
    setAlertsState({ editing })
  },
  setMeterEditing(editing: Meter | null) {
    const stats = appStore.state.meters.stats
    setMetersState({
      editing,
      editDimensions: editing ? meterDimensionDraftsFromMeter(editing, meterHasUsage(editing, stats)) : [],
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
    setUsageState({ filterQuery, selectedUsageEvent: null })
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
  setSelectedUsageEvent(event: UsageEvent | null) {
    setUsageState({ selectedUsageEvent: event })
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
    setUsageState({ error: '', events: [], eventsError: '', eventsStatus: 'idle', exportError: '', selectedUsageEvent: null, status: 'loading' })
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
  async loadCurrentUsageEvents(limit = 500) {
    setUsageState({ eventsError: '', eventsStatus: 'loading', selectedUsageEvent: null })
    try {
      const events = await listUsageEvents(currentUsageEventQuery(limit))
      setUsageState({ events: events.items, eventsStatus: 'ready' })
    } catch (err) {
      setUsageState({ eventsError: errorMessage(err, 'Unable to load usage events'), eventsStatus: 'error' })
    }
  },
  async exportCurrentUsageBuckets(groupByValue: string[], limit = 500, bucketSize = 'day') {
    setUsageState({ exportError: '', exporting: 'buckets' })
    try {
      const query = appStore.state.usage.filterQuery
      const scope = usageScopeFromQuery(query)
      const timeRange = usageTimeRangeFromQuery(query)
      const filter = usageFilterFromQuery(query, metadataTypesByField(appStore.state.usage.meters, scope.meter))
      const groupBy = groupByValue.filter(Boolean)
      const blob = await exportUsageBuckets({
        bucket_size: bucketSize,
        filter,
        from: timeRange.from,
        group_by: groupBy.length > 0 ? groupBy : undefined,
        limit,
        meter: scope.meter,
        subject: scope.subject || undefined,
        to: timeRange.to,
      })
      const fileParts = ['usage-buckets', scope.meter, scope.subject].filter((part): part is string => Boolean(part))
      downloadBlob(blob, `${fileParts.map(safeDownloadName).join('-')}.csv`)
    } catch (err) {
      setUsageState({ exportError: errorMessage(err, 'Unable to export usage buckets') })
    } finally {
      setUsageState({ exporting: '' })
    }
  },
  async exportCurrentUsageEvents(limit = 500) {
    setUsageState({ exportError: '', exporting: 'events' })
    try {
      const eventQuery = currentUsageEventQuery(limit)
      const blob = await exportUsageEvents(eventQuery)
      const fileParts = ['usage-events', eventQuery.meter, eventQuery.subject].filter((part): part is string => Boolean(part))
      downloadBlob(blob, `${fileParts.map(safeDownloadName).join('-')}.csv`)
    } catch (err) {
      setUsageState({ exportError: errorMessage(err, 'Unable to export usage events') })
    } finally {
      setUsageState({ exporting: '' })
    }
  },
  async queueCurrentUsageExport(groupByValue: string[], limit = 500, bucketSize = 'day') {
    setUsageState({ exportError: '', exportJobError: '', exporting: 'job' })
    try {
      const query = currentUsageBucketExportQuery(groupByValue, limit, bucketSize)
      const job = await createUsageExportJob({
        format: 'csv',
        kind: 'usage_buckets',
        query,
      })
      setUsageState((state) => ({
        exportJobStatus: 'ready',
        exportJobs: [job, ...state.exportJobs.filter((item) => item.id !== job.id)].slice(0, 8),
      }))
      await appStoreActions.loadUsageExportJobs()
    } catch (err) {
      setUsageState({ exportJobError: errorMessage(err, 'Unable to queue usage export') })
    } finally {
      setUsageState({ exporting: '' })
    }
  },
  async downloadUsageExport(job: UsageExportJob) {
    if (job.status !== 'completed') {
      return
    }

    setUsageState({ exportJobDownloading: job.id, exportJobError: '' })
    try {
      const blob = await downloadUsageExportJob(job)
      downloadBlob(blob, exportJobDownloadName(job))
    } catch (err) {
      setUsageState({ exportJobError: exportDownloadErrorMessage(err) })
    } finally {
      setUsageState({ exportJobDownloading: '' })
    }
  },
  async cancelUsageExport(job: UsageExportJob) {
    if (job.status !== 'queued' && job.status !== 'running') {
      return
    }

    setUsageState({ exportJobError: '', exportJobMutating: job.id })
    try {
      const updated = await cancelUsageExportJob(job.id)
      setUsageState((state) => ({ exportJobs: upsertUsageExportJob(state.exportJobs, updated) }))
    } catch (err) {
      setUsageState({ exportJobError: errorMessage(err, 'Unable to cancel export job') })
    } finally {
      setUsageState({ exportJobMutating: '' })
    }
  },
  async retryUsageExport(job: UsageExportJob) {
    if (job.status !== 'failed' && job.status !== 'canceled') {
      return
    }

    setUsageState({ exportJobError: '', exportJobMutating: job.id })
    try {
      const updated = await retryUsageExportJob(job.id)
      setUsageState((state) => ({ exportJobs: upsertUsageExportJob(state.exportJobs, updated) }))
      await appStoreActions.loadUsageExportJobs(appStore.state.usage.exportJobLimit)
    } catch (err) {
      setUsageState({ exportJobError: errorMessage(err, 'Unable to retry export job') })
    } finally {
      setUsageState({ exportJobMutating: '' })
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

function authErrorMessage(err: unknown, fallback: string) {
  if (err instanceof APIError && (err.status === 401 || err.code === 'unauthorized')) {
    return 'Email or password is incorrect.'
  }

  return errorMessage(err, fallback)
}

function registerErrorMessage(err: unknown, fallback: string) {
  if (err instanceof APIError && (err.status === 409 || err.code === 'conflict')) {
    return 'An account with this email already exists.'
  }
  if (err instanceof APIError && (err.status === 400 || err.code === 'invalid_input')) {
    const message = err.message.toLowerCase()
    if (message.includes('password')) {
      return 'Password must be at least 8 characters.'
    }
    if (message.includes('email')) {
      return 'Enter a valid email address.'
    }
    return 'Check your email and password, then try again.'
  }

  return errorMessage(err, fallback)
}

function exportDownloadErrorMessage(err: unknown) {
  if (err instanceof APIError && err.status === 404 && err.message.toLowerCase().includes('artifact')) {
    return 'This export file is no longer available. Queue a new export to generate it again.'
  }

  return errorMessage(err, 'Unable to download export')
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

function setAlertsState(update: Partial<AppState['alerts']> | ((state: AppState['alerts']) => Partial<AppState['alerts']>)) {
  appStore.setState((state) => ({
    ...state,
    alerts: {
      ...state.alerts,
      ...(typeof update === 'function' ? update(state.alerts) : update),
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

function currentUsageBucketExportQuery(groupByValue: string[], limit = 500, bucketSize = 'day'): UsageBucketExportQuery {
  const query = appStore.state.usage.filterQuery
  const scope = usageScopeFromQuery(query)
  const timeRange = usageTimeRangeFromQuery(query)
  const filter = usageFilterFromQuery(query, metadataTypesByField(appStore.state.usage.meters, scope.meter))
  const groupBy = groupByValue.filter(Boolean)
  return {
    bucket_size: bucketSize,
    filter,
    from: timeRange.from,
    group_by: groupBy.length > 0 ? groupBy : undefined,
    limit,
    meter: scope.meter,
    subject: scope.subject || undefined,
    to: timeRange.to,
  }
}

function currentUsageEventQuery(limit = 500): UsageEventQuery {
  const query = appStore.state.usage.filterQuery
  const scope = usageScopeFromQuery(query)
  const timeRange = usageTimeRangeFromQuery(query)
  const filter = usageFilterFromQuery(query, metadataTypesByField(appStore.state.usage.meters, scope.meter))
  return {
    filter,
    from: timeRange.from,
    limit,
    meter: scope.meter,
    subject: scope.subject || undefined,
    to: timeRange.to,
  }
}

function exportJobDownloadName(job: UsageExportJob) {
  const fileParts = ['usage-export', job.query.meter, job.id].filter((part): part is string => Boolean(part))
  return `${fileParts.map(safeDownloadName).join('-')}.csv`
}

function upsertUsageExportJob(items: UsageExportJob[], job: UsageExportJob) {
  return items.map((item) => item.id === job.id ? job : item)
}

function usageStateFromSavedQuery(query: SavedUsageQuery, state: AppState['usage']): Partial<AppState['usage']> {
  return {
    bucketSize: query.bucket_size || 'day',
    filterQuery: queryFromSavedValue(query.query, state.filterQuery),
    groupBy: query.group_by || [],
    limit: query.limit || 500,
    savedQueryName: query.name,
    selectedSavedQueryID: query.id,
    selectedUsageEvent: null,
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

function newMeterDimensionDraft(
  name = '',
  type = 'string',
  displayName = '',
  description = '',
  required = true,
  deprecated = false,
  original?: { deprecated: boolean; name: string; required: boolean; type: string },
): MeterDimensionDraft {
  meterDimensionID += 1
  return {
    deprecated,
    description,
    displayName,
    id: `meter-dimension-${meterDimensionID}`,
    name,
    originalDeprecated: original?.deprecated,
    originalName: original?.name,
    originalRequired: original?.required,
    originalType: original?.type,
    required,
    type,
  }
}

function meterDimensionDraftsFromMeter(meter: Meter, lockedByUsage = false) {
  const dimensions = normalizedMeterDimensions(meter)
  if (dimensions.length > 0) {
    return dimensions.map((dimension) => newMeterDimensionDraft(
      dimension.name,
      dimension.type,
      dimension.display_name,
      dimension.description,
      dimension.required,
      dimension.deprecated,
      {
        deprecated: dimension.deprecated,
        name: dimension.name,
        required: dimension.required,
        type: dimension.type,
      },
    ))
  }
  return meterDimensionDraftsFromSchema(meter.metadata_schema, lockedByUsage)
}

function meterDimensionDraftsFromSchema(schema: Record<string, string>, lockedByUsage = false) {
  const rows = Object.entries(schema || {})
    .sort(([left], [right]) => left.localeCompare(right))
    .map(([name, type]) => newMeterDimensionDraft(name, type, '', '', true, false, {
      deprecated: false,
      name,
      required: true,
      type,
    }))
  return rows.length > 0 ? rows : [newMeterDimensionDraft('', 'string', '', '', !lockedByUsage)]
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
      deprecated: false,
      name,
      required: true,
      type,
    }))
}

function meterHasUsage(meter: Meter | null, stats: Record<string, MeterStats>) {
  return meter ? (stats[meter.name]?.usage_events ?? 0) > 0 : false
}

function humanizeDimensionName(name: string) {
  return name
    .replace(/^metadata\./, '')
    .split(/[._-]/)
    .filter(Boolean)
    .map((part) => part.charAt(0).toUpperCase() + part.slice(1))
    .join(' ')
}
