import { createStore } from '@tanstack/react-store'
import type { RuleGroupType } from 'react-querybuilder'

import {
  createAlertDestination as createAlertDestinationRequest,
  createAlertRule,
  APIError,
  cancelUsageExportJob,
  createAPIKey as createAPIKeyRequest,
  createAuthSession,
  createAuthUser,
  createMeter as createMeterRequest,
  createPlan as createPlanRequest,
  createSavedUsageQuery,
  createUsageExportJob,
  deleteAlertRule,
  deleteAlertDestination as deleteAlertDestinationRequest,
  deleteAPIKey as deleteAPIKeyRequest,
  deleteAuthSession,
  deleteMeter as deleteMeterRequest,
  deletePlan as deletePlanRequest,
  deleteSubjectPlanAssignment as deleteSubjectPlanAssignmentRequest,
  deleteSavedUsageQuery,
  downloadUsageExportJob,
  exportUsageBuckets,
  exportUsageEvents,
  evaluateAlertRule,
  getSystemStats,
  getSubjectPlanProgress,
  assignSubjectPlan as assignSubjectPlanRequest,
  listAlertEvents,
  listAlertDestinations,
  listAlertRules,
  listAPIKeys,
  listIngestions,
  listMeterStats,
  listMeters,
  listOAuthProviders,
  listPlanAssignments,
  listPlans,
  listSavedUsageQueries,
  listSubjectEvents,
  listSubjects,
  listUsageBreakdowns,
  listUsageBuckets,
  listUsageDimensionValues,
  listUsageEvents,
  listUsageExportJobs,
  refreshAuthSession,
  rotateAlertDestinationSecret as rotateAlertDestinationSecretRequest,
  retryUsageExportJob,
  updatePlan as updatePlanRequest,
  updateAlertDestination as updateAlertDestinationRequest,
  updateAlertRule,
  updateMeter as updateMeterRequest,
  updateSavedUsageQuery,
  type AlertDestination,
  type AlertDestinationRequest,
  type AlertDestinationUpdateRequest,
  type AlertEvent,
  type AlertRule,
  type AlertRuleRequest,
  type AlertRuleUpdateRequest,
  type APIKey,
  type APIKeyCreateRequest,
  type APIKeyCreateResponse,
  type AuthSession,
  type Meter,
  type MeterCreateRequest,
  type MeterStats,
  type MeterUpdateRequest,
  type OAuthProvider,
  type Plan,
  type PlanAssignment,
  type PlanSaveRequest,
  type SavedUsageQuery,
  type SubjectPlanProgress,
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

type AlertWebhookSecret = {
  algorithm: string
  ownerID: string
  ownerName: string
  secret: string
  signatureHeader: string
  timestampHeader: string
}

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
    providers: OAuthProvider[]
    registerError: string
    session: AuthSession | null
  }
  apiKeys: {
    creating: boolean
    createdKey: APIKeyCreateResponse | null
    deleting: APIKey | null
    error: string
    items: APIKey[]
    saving: boolean
    status: LoadState
  }
  alerts: {
    creating: boolean
    destinationCreating: boolean
    destinationDeleting: AlertDestination | null
    destinationEditing: AlertDestination | null
    destinations: AlertDestination[]
    deleting: AlertRule | null
    editing: AlertRule | null
    error: string
    events: AlertEvent[]
    eventLoadingMore: boolean
    eventNextCursor: string
    eventStatus: LoadState
    items: AlertRule[]
    meters: Meter[]
    saving: boolean
    selectedEvent: AlertEvent | null
    signingSecret: AlertWebhookSecret | null
    status: LoadState
  }
  meters: {
    creating: boolean
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
  plans: {
    assignments: PlanAssignment[]
    assigning: boolean
    creating: boolean
    deleting: Plan | null
    editing: Plan | null
    error: string
    items: Plan[]
    meters: Meter[]
    progress: SubjectPlanProgress | null
    progressStatus: LoadState
    progressSubject: string
    saving: boolean
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
    loadingMore: boolean
    nextCursor: string
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
    exportJobLoadingMore: boolean
    exportJobLimit: number
    exportJobMutating: string
    exportJobNextCursor: string
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

type UserDataState = Pick<AppState, 'apiKeys' | 'alerts' | 'meters' | 'overview' | 'plans' | 'subjects' | 'usage'>

let meterDimensionID = 0
const domainSubjectField = 'subject'
const alertEventPageSize = 25
const subjectPageSize = 50
const exportJobPageSize = 50
let userDataGeneration = 0

export const appStore = createStore<AppState>({
  auth: {
    checked: false,
    loading: false,
    loginError: '',
    providers: [],
    registerError: '',
    session: null,
  },
  apiKeys: {
    creating: false,
    createdKey: null,
    deleting: null,
    error: '',
    items: [],
    saving: false,
    status: 'idle',
  },
  alerts: {
    creating: false,
    destinationCreating: false,
    destinationDeleting: null,
    destinationEditing: null,
    destinations: [],
    deleting: null,
    editing: null,
    error: '',
    events: [],
    eventLoadingMore: false,
    eventNextCursor: '',
    eventStatus: 'idle',
    items: [],
    meters: [],
    saving: false,
    selectedEvent: null,
    signingSecret: null,
    status: 'idle',
  },
  meters: {
    creating: false,
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
  plans: {
    assignments: [],
    assigning: false,
    creating: false,
    deleting: null,
    editing: null,
    error: '',
    items: [],
    meters: [],
    progress: null,
    progressStatus: 'idle',
    progressSubject: '',
    saving: false,
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
    loadingMore: false,
    nextCursor: '',
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
    exportJobLoadingMore: false,
    exportJobLimit: 8,
    exportJobMutating: '',
    exportJobNextCursor: '',
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

function currentUserDataGeneration() {
  return userDataGeneration
}

function isCurrentUserDataGeneration(generation: number) {
  return generation === userDataGeneration
}

function resetUserDataState() {
  userDataGeneration += 1
  appStore.setState((state) => ({
    ...state,
    ...initialUserDataState(),
  }))
}

function setAuthSession(update: Omit<Partial<AppState['auth']>, 'session'> & { session: AuthSession | null }) {
  const previousUserID = appStore.state.auth.session?.user.id ?? ''
  const nextUserID = update.session?.user.id ?? ''
  if (previousUserID !== nextUserID) {
    resetUserDataState()
  }
  setAuthState(update)
}

function initialUserDataState(): UserDataState {
  return {
    apiKeys: {
      creating: false,
      createdKey: null,
      deleting: null,
      error: '',
      items: [],
      saving: false,
      status: 'idle',
    },
    alerts: {
      creating: false,
      destinationCreating: false,
      destinationDeleting: null,
      destinationEditing: null,
      destinations: [],
      deleting: null,
      editing: null,
      error: '',
      events: [],
      eventLoadingMore: false,
      eventNextCursor: '',
      eventStatus: 'idle',
      items: [],
      meters: [],
      saving: false,
      selectedEvent: null,
      signingSecret: null,
      status: 'idle',
    },
    meters: {
      creating: false,
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
    plans: {
      assignments: [],
      assigning: false,
      creating: false,
      deleting: null,
      editing: null,
      error: '',
      items: [],
      meters: [],
      progress: null,
      progressStatus: 'idle',
      progressSubject: '',
      saving: false,
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
      loadingMore: false,
      nextCursor: '',
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
      exportJobLoadingMore: false,
      exportJobLimit: 8,
      exportJobMutating: '',
      exportJobNextCursor: '',
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
  }
}

export const appStoreActions = {
  clearCreatedAPIKey() {
    setAPIKeysState({ createdKey: null })
  },
  clearAlertSigningSecret() {
    setAlertsState({ signingSecret: null })
  },
  async createAPIKey(input: APIKeyCreateRequest) {
    setAPIKeysState({ createdKey: null, error: '', saving: true })
    try {
      const createdKey = await createAPIKeyRequest(input)
      setAPIKeysState({ createdKey })
      await appStoreActions.loadAPIKeys()
      setAPIKeysState({ creating: false })
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
      setMetersState({ creating: false })
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
    const generation = currentUserDataGeneration()
    setAlertsState({ error: '', eventLoadingMore: false, eventStatus: 'loading', status: 'loading' })
    try {
      const [meters, rules, events, destinations] = await Promise.all([
        listMeters(),
        listAlertRules(),
        listAlertEvents(alertEventPageSize),
        listAlertDestinations(),
      ])
      if (!isCurrentUserDataGeneration(generation)) {
        return
      }
      setAlertsState((state) => ({
        destinations: destinations.items,
        events: events.items,
        eventNextCursor: events.next_cursor || '',
        eventStatus: 'ready',
        items: rules.items,
        meters: meters.items,
        selectedEvent: state.selectedEvent ? events.items.find((event) => event.id === state.selectedEvent?.id) ?? null : null,
        status: 'ready',
      }))
    } catch (err) {
      if (!isCurrentUserDataGeneration(generation)) {
        return
      }
      setAlertsState({
        error: errorMessage(err, 'Unable to load alerts'),
        eventStatus: 'error',
        status: 'error',
      })
    }
  },
  async loadAlertEvents(options: { quiet?: boolean } = {}) {
    const generation = currentUserDataGeneration()
    if (!options.quiet) {
      setAlertsState({ error: '', eventStatus: 'loading' })
    }

    try {
      const events = await listAlertEvents(alertEventPageSize)
      if (!isCurrentUserDataGeneration(generation)) {
        return
      }
      setAlertsState((state) => ({
        events: options.quiet && state.events.length > events.items.length ? mergeAlertEvents(events.items, state.events) : events.items,
        eventNextCursor: options.quiet && state.events.length > events.items.length ? state.eventNextCursor : events.next_cursor || '',
        eventStatus: 'ready',
        selectedEvent: state.selectedEvent ? events.items.find((event) => event.id === state.selectedEvent?.id) ?? state.selectedEvent : null,
      }))
    } catch (err) {
      if (!isCurrentUserDataGeneration(generation)) {
        return
      }
      setAlertsState({
        error: errorMessage(err, 'Unable to load alert events'),
        eventStatus: 'error',
      })
    }
  },
  async loadMoreAlertEvents() {
    const cursor = appStore.state.alerts.eventNextCursor
    if (!cursor || appStore.state.alerts.eventLoadingMore) {
      return
    }

    const generation = currentUserDataGeneration()
    setAlertsState({ error: '', eventLoadingMore: true })
    try {
      const events = await listAlertEvents(alertEventPageSize, cursor)
      if (!isCurrentUserDataGeneration(generation)) {
        return
      }
      setAlertsState((state) => ({
        events: appendUniqueByKey(state.events, events.items, (event) => event.id),
        eventNextCursor: events.next_cursor || '',
      }))
    } catch (err) {
      if (!isCurrentUserDataGeneration(generation)) {
        return
      }
      setAlertsState({ error: errorMessage(err, 'Unable to load more alert events') })
    } finally {
      if (isCurrentUserDataGeneration(generation)) {
        setAlertsState({ eventLoadingMore: false })
      }
    }
  },
  async createAlert(input: AlertRuleRequest) {
    setAlertsState({ error: '', saving: true })
    try {
      await createAlertRule(input)
      await appStoreActions.loadAlerts()
      setAlertsState({ creating: false })
    } catch (err) {
      setAlertsState({ error: errorMessage(err, 'Unable to create alert') })
      throw err
    } finally {
      setAlertsState({ saving: false })
    }
  },
  async createAlertDestination(input: AlertDestinationRequest) {
    setAlertsState({ error: '', saving: true })
    try {
      const created = await createAlertDestinationRequest(input)
      const signingSecret = alertWebhookSecretFromDestination(created)
      setAlertsState((state) => ({
        destinationCreating: false,
        destinations: [...state.destinations, created],
        signingSecret: signingSecret ?? state.signingSecret,
      }))
    } catch (err) {
      setAlertsState({ error: errorMessage(err, 'Unable to create alert destination') })
      throw err
    } finally {
      setAlertsState({ saving: false })
    }
  },
  async rotateAlertDestinationSecret(destination: AlertDestination) {
    setAlertsState({ error: '', saving: true })
    try {
      const updated = await rotateAlertDestinationSecretRequest(destination.id)
      const signingSecret = alertWebhookSecretFromDestination(updated)
      setAlertsState((state) => ({
        destinations: state.destinations.map((item) => item.id === updated.id ? updated : item),
        items: state.items.map((item) => item.destination_id === updated.id ? { ...item, destination: updated } : item),
        signingSecret: signingSecret ?? state.signingSecret,
      }))
    } catch (err) {
      setAlertsState({ error: errorMessage(err, 'Unable to rotate destination signing secret') })
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
  async updateEditingAlertDestination(input: AlertDestinationUpdateRequest) {
    const editing = appStore.state.alerts.destinationEditing
    if (!editing) {
      return
    }

    setAlertsState({ error: '', saving: true })
    try {
      const updated = await updateAlertDestinationRequest(editing.id, input)
      setAlertsState((state) => ({
        destinationEditing: null,
        destinations: state.destinations.map((item) => item.id === updated.id ? updated : item),
        items: state.items.map((item) => item.destination_id === updated.id ? { ...item, destination: updated } : item),
      }))
    } catch (err) {
      setAlertsState({ error: errorMessage(err, 'Unable to update alert destination') })
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
        selectedEvent: state.selectedEvent?.rule_id === deleting.id ? null : state.selectedEvent,
        signingSecret: state.signingSecret?.ownerID === deleting.id ? null : state.signingSecret,
      }))
    } catch (err) {
      setAlertsState({ error: errorMessage(err, 'Unable to delete alert') })
      throw err
    } finally {
      setAlertsState({ saving: false })
    }
  },
  async deleteSelectedAlertDestination() {
    const deleting = appStore.state.alerts.destinationDeleting
    if (!deleting) {
      return
    }

    setAlertsState({ error: '', saving: true })
    try {
      await deleteAlertDestinationRequest(deleting.id)
      setAlertsState((state) => ({
        destinationDeleting: null,
        destinations: state.destinations.filter((item) => item.id !== deleting.id),
        signingSecret: state.signingSecret?.ownerID === deleting.id ? null : state.signingSecret,
      }))
    } catch (err) {
      setAlertsState({ error: errorMessage(err, 'Unable to delete alert destination') })
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
        events: mergeAlertEvents(result.events?.length ? result.events : result.event ? [result.event] : [], state.events),
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
      const [session, providers] = await Promise.all([refreshAuthSession(), listOAuthProviders()])
      setAuthSession({ checked: true, loading: false, providers: providers.items, session })
      return session?.user ?? null
    } catch {
      setAuthSession({ checked: true, loading: false, session: null })
      return null
    }
  },
  async loadAPIKeys() {
    const generation = currentUserDataGeneration()
    setAPIKeysState({ error: '', status: 'loading' })
    try {
      const keys = await listAPIKeys()
      if (!isCurrentUserDataGeneration(generation)) {
        return
      }
      setAPIKeysState({ items: keys.items, status: 'ready' })
    } catch (err) {
      if (!isCurrentUserDataGeneration(generation)) {
        return
      }
      setAPIKeysState({ error: errorMessage(err, 'Unable to load API keys'), status: 'error' })
    }
  },
  async loadMeters() {
    const generation = currentUserDataGeneration()
    setMetersState({ error: '', status: 'loading' })
    try {
      const [nextMeters, nextStats] = await Promise.all([listMeters(), listMeterStats()])
      if (!isCurrentUserDataGeneration(generation)) {
        return
      }
      setMetersState({
        items: nextMeters.items,
        stats: Object.fromEntries(nextStats.items.map((item) => [item.meter, item])),
        status: 'ready',
      })
    } catch (err) {
      if (!isCurrentUserDataGeneration(generation)) {
        return
      }
      setMetersState({ error: errorMessage(err, 'Unable to load meters'), status: 'error' })
    }
  },
  async loadPlans() {
    const generation = currentUserDataGeneration()
    setPlansState({ error: '', progressStatus: appStore.state.plans.progressStatus === 'idle' ? 'idle' : appStore.state.plans.progressStatus, status: 'loading' })
    try {
      const [plans, assignments, meters] = await Promise.all([
        listPlans(),
        listPlanAssignments(),
        listMeters(),
      ])
      if (!isCurrentUserDataGeneration(generation)) {
        return
      }
      setPlansState({
        assignments: assignments.items,
        items: plans.items,
        meters: meters.items,
        status: 'ready',
      })
    } catch (err) {
      if (!isCurrentUserDataGeneration(generation)) {
        return
      }
      setPlansState({ error: errorMessage(err, 'Unable to load plans'), status: 'error' })
    }
  },
  async createPlan(input: PlanSaveRequest) {
    setPlansState({ error: '', saving: true })
    try {
      await createPlanRequest(input)
      await appStoreActions.loadPlans()
      setPlansState({ creating: false })
    } catch (err) {
      setPlansState({ error: errorMessage(err, 'Unable to create plan') })
      throw err
    } finally {
      setPlansState({ saving: false })
    }
  },
  async updateEditingPlan(input: PlanSaveRequest) {
    const editing = appStore.state.plans.editing
    if (!editing) {
      return
    }
    setPlansState({ error: '', saving: true })
    try {
      await updatePlanRequest(editing.id, input)
      await appStoreActions.loadPlans()
      setPlansState({ editing: null })
    } catch (err) {
      setPlansState({ error: errorMessage(err, 'Unable to update plan') })
      throw err
    } finally {
      setPlansState({ saving: false })
    }
  },
  async deleteSelectedPlan() {
    const deleting = appStore.state.plans.deleting
    if (!deleting) {
      return
    }
    setPlansState({ error: '', saving: true })
    try {
      await deletePlanRequest(deleting.id)
      setPlansState((state) => ({
        deleting: null,
        items: state.items.filter((plan) => plan.id !== deleting.id),
      }))
    } catch (err) {
      setPlansState({ error: errorMessage(err, 'Unable to delete plan') })
      throw err
    } finally {
      setPlansState({ saving: false })
    }
  },
  async assignSubjectPlan(subject: string, planID: string) {
    setPlansState({ assigning: true, error: '' })
    try {
      const assignment = await assignSubjectPlanRequest(subject, planID)
      setPlansState((state) => ({
        assignments: [assignment, ...state.assignments.filter((item) => item.subject !== assignment.subject)],
      }))
      if (appStore.state.plans.progressSubject === assignment.subject) {
        await appStoreActions.loadSubjectPlanProgress(assignment.subject)
      }
    } catch (err) {
      setPlansState({ error: errorMessage(err, 'Unable to assign plan') })
      throw err
    } finally {
      setPlansState({ assigning: false })
    }
  },
  async deleteSubjectPlanAssignment(subject: string) {
    setPlansState({ assigning: true, error: '' })
    try {
      await deleteSubjectPlanAssignmentRequest(subject)
      setPlansState((state) => ({
        assignments: state.assignments.filter((item) => item.subject !== subject),
        progress: state.progress?.subject === subject ? null : state.progress,
        progressStatus: state.progress?.subject === subject ? 'idle' : state.progressStatus,
      }))
    } catch (err) {
      setPlansState({ error: errorMessage(err, 'Unable to remove assignment') })
      throw err
    } finally {
      setPlansState({ assigning: false })
    }
  },
  async loadSubjectPlanProgress(subject = appStore.state.plans.progressSubject) {
    const normalized = subject.trim()
    if (!normalized) {
      setPlansState({ progress: null, progressStatus: 'idle', progressSubject: '' })
      return
    }
    const generation = currentUserDataGeneration()
    setPlansState({ error: '', progressStatus: 'loading', progressSubject: normalized })
    try {
      const progress = await getSubjectPlanProgress(normalized)
      if (!isCurrentUserDataGeneration(generation)) {
        return
      }
      setPlansState({ progress, progressStatus: 'ready' })
    } catch (err) {
      if (!isCurrentUserDataGeneration(generation)) {
        return
      }
      setPlansState({ error: errorMessage(err, 'Unable to load plan progress'), progress: null, progressStatus: 'error' })
    }
  },
  async loadOverview() {
    const generation = currentUserDataGeneration()
    setOverviewState({ error: '', status: 'loading' })
    try {
      const [nextStats, nextSubjects, nextIngestions] = await Promise.all([
        getSystemStats(),
        listSubjects(),
        listIngestions(),
      ])
      if (!isCurrentUserDataGeneration(generation)) {
        return
      }
      setOverviewState({
        ingestions: nextIngestions.items,
        stats: nextStats,
        status: 'ready',
        subjects: nextSubjects.items,
      })

      try {
        const [savedQueries, meters] = await Promise.all([
          listSavedUsageQueries(),
          listMeters(),
        ])
        const pinned = savedQueries.items
          .filter((query) => query.pinned)
          .sort((left, right) => left.position - right.position || left.name.localeCompare(right.name))
          .slice(0, 6)
        const pinnedUsageQueries = await Promise.all(pinned.map((query) => summarizePinnedUsageQuery(query, meters.items)))
        if (!isCurrentUserDataGeneration(generation)) {
          return
        }
        setOverviewState({ pinnedUsageQueries })
      } catch (err) {
        if (!isCurrentUserDataGeneration(generation)) {
          return
        }
        setOverviewState({ error: errorMessage(err, 'Unable to load pinned usage queries') })
      }
    } catch (err) {
      if (!isCurrentUserDataGeneration(generation)) {
        return
      }
      setOverviewState({ error: errorMessage(err, 'Unable to load overview'), status: 'error' })
    }
  },
  async loadSubjects(preferredSubject = '') {
    const generation = currentUserDataGeneration()
    setSubjectsState({ error: '', loadingMore: false, status: 'loading' })
    try {
      const subjects = await listSubjects(subjectPageSize)
      if (!isCurrentUserDataGeneration(generation)) {
        return
      }
      const selectedSubject = preferredSubject.trim() || selectedSubjectForList(appStore.state.subjects.selectedSubject, subjects.items)
      setSubjectsState({
        items: subjects.items,
        nextCursor: subjects.next_cursor || '',
        selectedSubject,
        status: 'ready',
      })
      if (selectedSubject) {
        await appStoreActions.loadSubjectEvents(selectedSubject)
      } else {
        setSubjectsState({ detailStatus: 'idle', events: [] })
      }
    } catch (err) {
      if (!isCurrentUserDataGeneration(generation)) {
        return
      }
      setSubjectsState({ error: errorMessage(err, 'Unable to load subjects'), status: 'error' })
    }
  },
  async loadMoreSubjects() {
    const cursor = appStore.state.subjects.nextCursor
    if (!cursor || appStore.state.subjects.loadingMore) {
      return
    }

    const generation = currentUserDataGeneration()
    setSubjectsState({ error: '', loadingMore: true })
    try {
      const subjects = await listSubjects(subjectPageSize, cursor)
      if (!isCurrentUserDataGeneration(generation)) {
        return
      }
      setSubjectsState((state) => ({
        items: appendUniqueByKey(state.items, subjects.items, (subject) => subject.subject),
        nextCursor: subjects.next_cursor || '',
      }))
    } catch (err) {
      if (!isCurrentUserDataGeneration(generation)) {
        return
      }
      setSubjectsState({ error: errorMessage(err, 'Unable to load more subjects') })
    } finally {
      if (isCurrentUserDataGeneration(generation)) {
        setSubjectsState({ loadingMore: false })
      }
    }
  },
  async loadSubjectEvents(subject: string) {
    if (!subject) {
      setSubjectsState({ detailStatus: 'idle', events: [], selectedSubject: '' })
      return
    }

    const generation = currentUserDataGeneration()
    setSubjectsState({ detailStatus: 'loading', error: '', exportError: '', selectedSubject: subject })
    try {
      const events = await listSubjectEvents(subject, 25)
      if (!isCurrentUserDataGeneration(generation)) {
        return
      }
      setSubjectsState({ detailStatus: 'ready', events })
    } catch (err) {
      if (!isCurrentUserDataGeneration(generation)) {
        return
      }
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
    const generation = currentUserDataGeneration()
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
        listUsageExportJobs(exportJobPageSize),
      ])
      if (!isCurrentUserDataGeneration(generation)) {
        return
      }
      setUsageState((state) => ({
        exportJobLimit: exportJobPageSize,
        exportJobLoadingMore: false,
        exportJobNextCursor: exportJobs.next_cursor || '',
        exportJobs: exportJobs.items,
        exportJobStatus: 'ready',
        meters: nextMeters.items,
        savedQueries: savedQueries.items,
        savedQueryStatus: 'ready',
        filterQuery: queryWithAvailableMeter(state.filterQuery, nextMeters.items),
        status: 'ready',
      }))
    } catch (err) {
      if (!isCurrentUserDataGeneration(generation)) {
        return
      }
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
  async loadUsageExportJobs(limit = appStore.state.usage.exportJobLimit || exportJobPageSize, options: { preserveLoaded?: boolean; quiet?: boolean } = {}) {
    const generation = currentUserDataGeneration()
    if (!options.quiet) {
      setUsageState({ exportJobError: '', exportJobStatus: 'loading' })
    }
    try {
      const exportJobs = await listUsageExportJobs(limit)
      if (!isCurrentUserDataGeneration(generation)) {
        return
      }
      setUsageState((state) => ({
        exportJobLimit: limit,
        exportJobNextCursor: options.preserveLoaded && state.exportJobs.length > exportJobs.items.length ? state.exportJobNextCursor : exportJobs.next_cursor || '',
        exportJobs: options.preserveLoaded && state.exportJobs.length > exportJobs.items.length ? mergeByID(exportJobs.items, state.exportJobs) : exportJobs.items,
        exportJobStatus: 'ready',
      }))
    } catch (err) {
      if (!isCurrentUserDataGeneration(generation)) {
        return
      }
      setUsageState({
        exportJobError: errorMessage(err, 'Unable to load export jobs'),
        exportJobStatus: 'error',
      })
    }
  },
  async loadMoreUsageExportJobs(limit = appStore.state.usage.exportJobLimit || exportJobPageSize) {
    const cursor = appStore.state.usage.exportJobNextCursor
    if (!cursor || appStore.state.usage.exportJobLoadingMore) {
      return
    }

    const generation = currentUserDataGeneration()
    setUsageState({ exportJobError: '', exportJobLoadingMore: true })
    try {
      const exportJobs = await listUsageExportJobs(limit, cursor)
      if (!isCurrentUserDataGeneration(generation)) {
        return
      }
      setUsageState((state) => ({
        exportJobs: appendUniqueByKey(state.exportJobs, exportJobs.items, (job) => job.id),
        exportJobNextCursor: exportJobs.next_cursor || '',
      }))
    } catch (err) {
      if (!isCurrentUserDataGeneration(generation)) {
        return
      }
      setUsageState({ exportJobError: errorMessage(err, 'Unable to load more export jobs') })
    } finally {
      if (isCurrentUserDataGeneration(generation)) {
        setUsageState({ exportJobLoadingMore: false })
      }
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

    const generation = currentUserDataGeneration()
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
      if (!isCurrentUserDataGeneration(generation)) {
        return
      }
      setUsageState({ dimensionValues: Object.fromEntries(values) })
    } catch {
      if (!isCurrentUserDataGeneration(generation)) {
        return
      }
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

    const generation = currentUserDataGeneration()
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
      if (!isCurrentUserDataGeneration(generation)) {
        return
      }
      setUsageState({ breakdowns: Object.fromEntries(breakdowns), breakdownStatus: 'ready' })
    } catch (err) {
      if (!isCurrentUserDataGeneration(generation)) {
        return
      }
      setUsageState({
        breakdownError: errorMessage(err, 'Unable to load usage breakdowns'),
        breakdowns: {},
        breakdownStatus: 'error',
      })
    }
  },
  async login(input: { email: string; password: string }) {
    resetUserDataState()
    setAuthState({ loading: true, loginError: '', registerError: '', session: null })
    try {
      const session = await createAuthSession(input)
      setAuthSession({ checked: true, loading: false, session })
      return session
    } catch (err) {
      setAuthSession({
        checked: true,
        loading: false,
        loginError: authErrorMessage(err, 'Unable to sign in'),
        session: null,
      })
      throw err
    }
  },
  async logout() {
    resetUserDataState()
    setAuthState({ loading: true, session: null })
    try {
      await deleteAuthSession()
    } finally {
      setAuthState({ checked: true, loading: false, loginError: '', registerError: '', session: null })
    }
  },
  async register(input: { email: string; password: string }) {
    resetUserDataState()
    setAuthState({ loading: true, loginError: '', registerError: '', session: null })
    try {
      await createAuthUser(input)
      const session = await createAuthSession(input)
      setAuthSession({ checked: true, loading: false, session })
      return session
    } catch (err) {
      setAuthSession({
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
  setMeterCreating(creating: boolean) {
    setMetersState({ creating })
  },
  setSubjectSearchQuery(searchQuery: string) {
    setSubjectsState({ searchQuery })
  },
  setMeterDeleting(deleting: Meter | null) {
    setMetersState({ deleting })
  },
  setPlanCreating(creating: boolean) {
    setPlansState({ creating })
  },
  setPlanEditing(editing: Plan | null) {
    setPlansState({ editing })
  },
  setPlanDeleting(deleting: Plan | null) {
    setPlansState({ deleting })
  },
  setPlanProgressSubject(progressSubject: string) {
    setPlansState({ progressSubject })
  },
  setAPIKeyCreating(creating: boolean) {
    setAPIKeysState({ creating })
  },
  setAPIKeyDeleting(deleting: APIKey | null) {
    setAPIKeysState({ deleting })
  },
  setAlertDeleting(deleting: AlertRule | null) {
    setAlertsState({ deleting })
  },
  setAlertCreating(creating: boolean) {
    setAlertsState({ creating })
  },
  setAlertDestinationDeleting(destinationDeleting: AlertDestination | null) {
    setAlertsState({ destinationDeleting })
  },
  setAlertDestinationCreating(destinationCreating: boolean) {
    setAlertsState({ destinationCreating })
  },
  setAlertDestinationEditing(destinationEditing: AlertDestination | null) {
    setAlertsState({ destinationEditing })
  },
  setAlertEditing(editing: AlertRule | null) {
    setAlertsState({ editing })
  },
  setAlertSelectedEvent(selectedEvent: AlertEvent | null) {
    setAlertsState({ selectedEvent })
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
    const generation = currentUserDataGeneration()
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
      if (!isCurrentUserDataGeneration(generation)) {
        return
      }
      setUsageState({ buckets, status: 'ready' })
    } catch (err) {
      if (!isCurrentUserDataGeneration(generation)) {
        return
      }
      setUsageState({ error: errorMessage(err, 'Unable to query usage'), status: 'error' })
    }
  },
  async loadCurrentUsageEvents(limit = 500) {
    const generation = currentUserDataGeneration()
    setUsageState({ eventsError: '', eventsStatus: 'loading', selectedUsageEvent: null })
    try {
      const events = await listUsageEvents(currentUsageEventQuery(limit))
      if (!isCurrentUserDataGeneration(generation)) {
        return
      }
      setUsageState({ events: events.items, eventsStatus: 'ready' })
    } catch (err) {
      if (!isCurrentUserDataGeneration(generation)) {
        return
      }
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
    const generation = currentUserDataGeneration()
    setUsageState({ exportError: '', exportJobError: '', exporting: 'job' })
    try {
      const query = currentUsageBucketExportQuery(groupByValue, limit, bucketSize)
      const job = await createUsageExportJob({
        format: 'csv',
        kind: 'usage_buckets',
        query,
      })
      if (!isCurrentUserDataGeneration(generation)) {
        return
      }
      setUsageState((state) => ({
        exportJobStatus: 'ready',
        exportJobs: mergeByID([job], state.exportJobs),
      }))
      await appStoreActions.loadUsageExportJobs(appStore.state.usage.exportJobLimit, { preserveLoaded: true, quiet: true })
    } catch (err) {
      if (!isCurrentUserDataGeneration(generation)) {
        return
      }
      setUsageState({ exportJobError: errorMessage(err, 'Unable to queue usage export') })
    } finally {
      if (isCurrentUserDataGeneration(generation)) {
        setUsageState({ exporting: '' })
      }
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
      await appStoreActions.loadUsageExportJobs(appStore.state.usage.exportJobLimit, { preserveLoaded: true, quiet: true })
    } catch (err) {
      setUsageState({ exportJobError: errorMessage(err, 'Unable to retry export job') })
    } finally {
      setUsageState({ exportJobMutating: '' })
    }
  },
  async saveCurrentUsageQuery() {
    const state = appStore.state.usage
    const selectedID = state.selectedSavedQueryID
    const selectedQuery = state.savedQueries.find((item) => item.id === selectedID)
    const position = selectedQuery?.pinned ? selectedQuery.position : nextPinnedPosition(state.savedQueries, selectedID)
    setUsageState({ savedQueryError: '', savedQuerySaving: true })
    try {
      const input = {
        bucket_size: state.bucketSize,
        group_by: state.groupBy,
        limit: state.limit,
        name: state.savedQueryName,
        pinned: true,
        position,
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
  if (err instanceof APIError) {
    if (err.status === 401 || err.code === 'unauthorized') {
      return 'Your session has expired. Sign in again to continue.'
    }
    if (err.status === 403 || err.code === 'forbidden') {
      return 'You do not have access to this action in the current workspace.'
    }
  }

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

function mergeAlertEvents(next: AlertEvent[], current: AlertEvent[]) {
  return mergeByID(next, current)
}

function mergeByID<T extends { id: string }>(next: T[], current: T[]) {
  return appendUniqueByKey(next, current, (item) => item.id)
}

function appendUniqueByKey<T>(current: T[], next: T[], keyFor: (item: T) => string) {
  if (next.length === 0) {
    return current
  }
  const currentKeys = new Set(current.map(keyFor))
  return [...current, ...next.filter((item) => !currentKeys.has(keyFor(item)))]
}

function alertWebhookSecretFromDestination(destination: AlertDestination): AlertWebhookSecret | null {
  const secret = destination.webhook_signing?.secret
  if (!secret) {
    return null
  }
  return {
    algorithm: destination.webhook_signing.algorithm,
    ownerID: destination.id,
    ownerName: destination.name,
    secret,
    signatureHeader: destination.webhook_signing.signature_header,
    timestampHeader: destination.webhook_signing.timestamp_header,
  }
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

function setPlansState(update: Partial<AppState['plans']> | ((state: AppState['plans']) => Partial<AppState['plans']>)) {
  appStore.setState((state) => ({
    ...state,
    plans: {
      ...state.plans,
      ...(typeof update === 'function' ? update(state.plans) : update),
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

function setSubjectsState(update: Partial<AppState['subjects']> | ((state: AppState['subjects']) => Partial<AppState['subjects']>)) {
  appStore.setState((state) => ({
    ...state,
    subjects: {
      ...state.subjects,
      ...(typeof update === 'function' ? update(state.subjects) : update),
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
  if (meter.dimensions.length > 0) {
    return meter.dimensions.map((dimension) => newMeterDimensionDraft(
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
  return [newMeterDimensionDraft('', 'string', '', '', !lockedByUsage)]
}

function meterHasUsage(meter: Meter | null, stats: Record<string, MeterStats>) {
  return meter ? (stats[meter.name]?.usage_events ?? 0) > 0 : false
}
