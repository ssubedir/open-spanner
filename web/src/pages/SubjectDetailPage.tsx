import { useRouter } from '@tanstack/react-router'
import { useSelector } from '@tanstack/react-store'
import { ArrowLeft, BarChart3, Clock, Database, Download, Eye, Loader2, Users } from 'lucide-react'
import { useCallback, useEffect, useMemo } from 'react'

import { appStore, appStoreActions } from '../app-store'
import type { EntitlementEvent, SubjectStats, UsageEvent } from '../api'
import { DataTable, DetailLoadingPage, DetailStatePage, MetricCard, Modal, PageHeader } from '../components/dashboard'
import { EntitlementEventDetail, EntitlementEventType, EntitlementStateBadge } from '../components/entitlement-event-detail'
import { Badge } from '../components/ui/badge'
import { Button } from '../components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '../components/ui/card'
import { formatDate, formatNumber } from '../lib/format'
import { useInitialLoad } from '../lib/hooks'
import { isValidSubjectIdentifier, normalizeSubjectIdentifier } from '../lib/subjects'

export function SubjectDetailPage({ routeSubject }: { routeSubject: string }) {
  const router = useRouter()
  const {
    detailStatus,
    entitlementEventLoadingMore,
    entitlementEventNextCursor,
    entitlementEvents,
    entitlementStates,
    error,
    events,
    exportError,
    exporting,
    items,
    selectedEntitlementEvent,
    selectedSubject,
    status,
  } = useSelector(appStore, (state) => state.subjects)
  const selectedRouteSubject = normalizeSubjectIdentifier(routeSubject)
  const routeSubjectIsValid = isValidSubjectIdentifier(selectedRouteSubject)
  const subjectName = routeSubjectIsValid ? selectedSubject || selectedRouteSubject : selectedRouteSubject
  const load = useCallback(async () => {
    if (!routeSubjectIsValid) {
      await appStoreActions.loadSubjectEvents('')
      return
    }
    await appStoreActions.loadSubjects(selectedRouteSubject)
  }, [routeSubjectIsValid, selectedRouteSubject])

  useInitialLoad(load)

  useEffect(() => {
    if (!subjectName) {
      return undefined
    }
    const poll = window.setInterval(() => {
      void appStoreActions.loadSubjectEntitlementActivity(subjectName, { quiet: true })
    }, 5000)

    return () => window.clearInterval(poll)
  }, [subjectName])

  const selectedStats = items.find((subject) => subject.subject === subjectName) ?? null
  const meterSummaries = useMemo(() => summarizeMeters(events), [events])
  const loading = detailStatus === 'loading' || (status === 'loading' && !selectedSubject)
  const subjectExists = Boolean(selectedStats) || events.length > 0 || entitlementStates.length > 0 || entitlementEvents.length > 0
  const subjectPending = routeSubjectIsValid && !subjectExists && (loading || detailStatus === 'idle')
  const subjectNotFound = routeSubjectIsValid && status === 'ready' && detailStatus === 'ready' && !subjectExists

  function openUsageForSubject() {
    if (!subjectName) {
      return
    }
    appStoreActions.prepareUsageForSubject(subjectName, meterSummaries[0]?.meter ?? '')
    void router.navigate({ to: '/usage' })
  }

  async function exportSubjectEvents() {
    await appStoreActions.exportSelectedSubjectEvents()
  }

  if (subjectPending) {
    return (
      <DetailLoadingPage
        eyebrow="Subjects"
        icon={<Users />}
        title="Loading subject"
        description="Loading subject activity before showing usage and entitlement details."
        action={(
          <Button onClick={() => void router.navigate({ to: '/subjects' })} type="button" variant="outline">
            <ArrowLeft aria-hidden="true" />
            Back to subjects
          </Button>
        )}
      />
    )
  }

  if (!routeSubjectIsValid || subjectNotFound) {
    return <SubjectNotFoundPage subject={selectedRouteSubject} />
  }

  return (
    <>
      <PageHeader
        eyebrow="Subjects"
        icon={<Users />}
        title={subjectName || 'Subject detail'}
        description={detailDescription(selectedStats, detailStatus, subjectName)}
        action={(
          <div className="flex flex-wrap justify-end gap-2">
            <Button onClick={() => void router.navigate({ to: '/subjects' })} type="button" variant="outline">
              <ArrowLeft aria-hidden="true" />
              Back
            </Button>
            <Button disabled={!subjectName || loading} onClick={openUsageForSubject} type="button" variant="outline">
              <BarChart3 aria-hidden="true" />
              Open usage
            </Button>
            <Button disabled={!subjectName || exporting} onClick={() => void exportSubjectEvents()} type="button" variant="outline">
              {exporting ? <Loader2 className="spin" aria-hidden="true" /> : <Download aria-hidden="true" />}
              Export events
            </Button>
          </div>
        )}
      />

      {error ? <div className="error-banner">{error}</div> : null}
      {exportError ? <div className="inline-error mb-4">{exportError}</div> : null}

      <section className="mb-4 grid gap-4 md:grid-cols-3" aria-label="Selected subject summary">
        <MetricCard icon={<BarChart3 />} label="Usage Events" loading={loading} value={selectedStats?.usage_events ?? events.length} helper="Events for this subject" />
        <MetricCard icon={<Database />} label="Meters" loading={loading} value={selectedStats?.meters ?? meterSummaries.length} helper="Distinct subject meters" />
        <SubjectDateCard loading={loading} value={selectedStats?.last_event_at ? formatDate(selectedStats.last_event_at) : 'No events yet'} />
      </section>

      <div className="grid max-w-[1480px] gap-4">
        <section className="grid gap-4 xl:grid-cols-2">
          <Card className="min-w-0">
            <CardHeader className="!px-4 !py-3">
              <div>
                <CardTitle>Meter Activity</CardTitle>
                <CardDescription>Recent usage grouped by meter.</CardDescription>
              </div>
            </CardHeader>
            <CardContent>
              {meterSummaries.length > 0 ? (
                <div className="subject-meter-list">
                  {meterSummaries.map((meter) => (
                    <div className="subject-meter-row" key={meter.meter}>
                      <div>
                        <strong>{meter.meter}</strong>
                        <span>{formatNumber(meter.events)} events</span>
                      </div>
                      <strong>{formatNumber(meter.quantity)}</strong>
                    </div>
                  ))}
                </div>
              ) : (
                <p className="subject-empty">No recent meter activity.</p>
              )}
            </CardContent>
          </Card>

          <Card className="min-w-0">
            <CardHeader className="!px-4 !py-3">
              <div>
                <CardTitle>Entitlements</CardTitle>
                <CardDescription>Current quota state for this subject.</CardDescription>
              </div>
            </CardHeader>
            <CardContent>
              {entitlementStates.length > 0 ? (
                <div className="subject-meter-list">
                  {entitlementStates.map((state) => (
                    <div className="subject-meter-row" key={`${state.plan_id}-${state.meter}-${state.period}`}>
                      <div>
                        <strong>{state.meter}</strong>
                        <span>{state.plan_name} - {formatNumber(state.current)} / {formatNumber(state.limit)} {state.period}</span>
                      </div>
                      <EntitlementStateBadge state={state.state} />
                    </div>
                  ))}
                </div>
              ) : (
                <p className="subject-empty">No entitlement checks recorded for this subject yet.</p>
              )}
            </CardContent>
          </Card>
        </section>

        <Card className="min-w-0 subject-events-card">
          <CardHeader className="!px-4 !py-3">
            <div>
              <CardTitle>Recent Events</CardTitle>
              <CardDescription>Latest usage for {subjectName}.</CardDescription>
            </div>
          </CardHeader>
          <CardContent>
            <DataTable
              emptyLabel={loading ? 'Loading recent events' : 'No recent events for this subject'}
              headers={['Timestamp', 'Meter', 'Quantity', 'Metadata', 'ID']}
              rows={events.map((event) => [
                formatDate(event.timestamp),
                <Badge variant="muted">{event.meter}</Badge>,
                formatNumber(event.quantity),
                <MetadataValues metadata={event.metadata} />,
                <span className="mono truncate">{event.id}</span>,
              ])}
            />
          </CardContent>
        </Card>

        <Card className="min-w-0">
          <CardHeader className="!px-4 !py-3">
            <div>
              <CardTitle>Entitlement Changes</CardTitle>
              <CardDescription>Recent quota transitions for {subjectName}.</CardDescription>
            </div>
          </CardHeader>
          <CardContent>
            <EntitlementEventTable
              events={entitlementEvents}
              onSelect={(event) => appStoreActions.setSubjectSelectedEntitlementEvent(event)}
              selectedSubject={subjectName}
            />
            {entitlementEventNextCursor ? (
              <div className="pagination-actions">
                <Button disabled={entitlementEventLoadingMore} onClick={() => void appStoreActions.loadMoreSubjectEntitlementEvents()} type="button" variant="outline">
                  {entitlementEventLoadingMore ? <Loader2 className="spin" aria-hidden="true" /> : null}
                  Load more changes
                </Button>
              </div>
            ) : null}
          </CardContent>
        </Card>
      </div>

      {selectedEntitlementEvent ? (
        <Modal className="!w-full !max-w-[760px]" title="Entitlement Change" onClose={() => appStoreActions.setSubjectSelectedEntitlementEvent(null)}>
          <EntitlementEventDetail event={selectedEntitlementEvent} />
          <div className="modal-actions">
            <Button onClick={() => appStoreActions.setSubjectSelectedEntitlementEvent(null)} type="button" variant="outline">Close</Button>
          </div>
        </Modal>
      ) : null}
    </>
  )
}

function SubjectNotFoundPage({ subject }: { subject: string }) {
  const router = useRouter()

  return (
    <DetailStatePage
      icon={<Users />}
      title="Subject not found"
      description={subject ? `No subject activity exists for ${subject}.` : 'The subject route is invalid.'}
      action={(
        <Button onClick={() => void router.navigate({ to: '/subjects' })} type="button" variant="outline">
          <ArrowLeft aria-hidden="true" />
          Back to subjects
        </Button>
      )}
    />
  )
}

function MetadataValues({ metadata }: { metadata: Record<string, unknown> }) {
  const entries = Object.entries(metadata || {})
  if (entries.length === 0) {
    return <span className="muted">none</span>
  }

  return (
    <div className="dimension-values">
      {entries.slice(0, 4).map(([key, value]) => (
        <span className="dimension-value" key={key}>
          <span>{key}</span>
          <strong>{formatMetadataValue(value)}</strong>
        </span>
      ))}
      {entries.length > 4 ? <span className="muted">+{entries.length - 4}</span> : null}
    </div>
  )
}

function SubjectDateCard({ loading, value }: { loading: boolean; value: string }) {
  return (
    <Card className="metric-card">
      <div className="metric-icon"><Clock aria-hidden="true" /></div>
      <div>
        <span>Last Event</span>
        <strong aria-busy={loading}>
          {loading ? <Loader2 aria-label="Loading metric" className="metric-loading spin" /> : value}
        </strong>
        <small>Most recent usage event</small>
      </div>
    </Card>
  )
}

function EntitlementEventTable({ events, onSelect, selectedSubject }: { events: EntitlementEvent[]; onSelect: (event: EntitlementEvent) => void; selectedSubject: string }) {
  return (
    <DataTable
      emptyLabel={selectedSubject ? 'No entitlement changes for this subject' : 'Select a subject to view entitlement changes'}
      headers={['Type', 'Meter', 'Plan', 'Usage', 'Message', 'Created', 'Actions']}
      rows={events.map((event) => [
        <EntitlementEventType event={event} />,
        <Badge variant="muted">{event.meter}</Badge>,
        event.plan_name,
        <span>{formatNumber(event.current)} / {formatNumber(event.limit)}</span>,
        <span className="max-w-[460px] truncate">{event.message}</span>,
        formatDate(event.created_at),
        <span className="table-actions">
          <Button aria-label={`View ${event.type} entitlement change`} onClick={() => onSelect(event)} size="icon" type="button" variant="ghost">
            <Eye aria-hidden="true" />
          </Button>
        </span>,
      ])}
    />
  )
}

function summarizeMeters(events: UsageEvent[]) {
  const summaries = new Map<string, { events: number; meter: string; quantity: number }>()
  for (const event of events) {
    const current = summaries.get(event.meter) ?? { events: 0, meter: event.meter, quantity: 0 }
    current.events += 1
    current.quantity += event.quantity
    summaries.set(event.meter, current)
  }
  return Array.from(summaries.values()).sort((left, right) => right.quantity - left.quantity || left.meter.localeCompare(right.meter))
}

function detailDescription(subject: SubjectStats | null, status: string, selectedSubject: string) {
  if (status === 'loading') {
    return 'Loading recent activity.'
  }
  if (!selectedSubject) {
    return 'Subject activity and entitlement state.'
  }
  if (!subject) {
    return 'Recent activity for the linked subject.'
  }
  return `${formatNumber(subject.usage_events)} events across ${formatNumber(subject.meters)} meters.`
}

function formatMetadataValue(value: unknown) {
  if (typeof value === 'string') {
    return value
  }
  if (typeof value === 'number' || typeof value === 'boolean') {
    return String(value)
  }
  if (value == null) {
    return 'null'
  }
  return JSON.stringify(value)
}
