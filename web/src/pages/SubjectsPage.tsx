import { useParams, useRouter } from '@tanstack/react-router'
import { useSelector } from '@tanstack/react-store'
import { BarChart3, Clock, Database, Download, Hash, Loader2, Search, Users } from 'lucide-react'
import type React from 'react'
import { useCallback, useMemo } from 'react'

import { appStore, appStoreActions } from '../app-store'
import type { SubjectStats, UsageEvent } from '../api'
import { DataTable, MetricCard, PageHeader } from '../components/dashboard'
import { Badge } from '../components/ui/badge'
import { Button } from '../components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '../components/ui/card'
import { Input } from '../components/ui/input'
import { Label } from '../components/ui/label'
import { formatDate, formatNumber } from '../lib/format'
import { useInitialLoad } from '../lib/hooks'

type SubjectsPageProps = {
  routeSubject?: string
}

export function SubjectRoutePage() {
  const { subject } = useParams({ from: '/_dashboard/subjects_/$subject' })

  return <SubjectsPage routeSubject={subject} />
}

export function SubjectsPage({ routeSubject = '' }: SubjectsPageProps) {
  const router = useRouter()
  const {
    detailStatus,
    error,
    events,
    exportError,
    exporting,
    items,
    loadingMore,
    nextCursor,
    searchQuery,
    selectedSubject,
    status,
  } = useSelector(appStore, (state) => state.subjects)
  const selectedRouteSubject = routeSubject.trim()
  const load = useCallback(() => appStoreActions.loadSubjects(selectedRouteSubject), [selectedRouteSubject])

  useInitialLoad(load)

  const visibleSubjects = useMemo(
    () => filterSubjects(items, searchQuery),
    [items, searchQuery],
  )
  const metricsLoading = status === 'idle' || (status === 'loading' && items.length === 0)
  const selectedStats = items.find((subject) => subject.subject === selectedSubject) ?? null
  const meterSummaries = useMemo(() => summarizeMeters(events), [events])

  function selectSubject(subject: string) {
    void router.navigate({ to: '/subjects/$subject', params: { subject } })
    void appStoreActions.loadSubjectEvents(subject)
  }

  function openUsageForSubject() {
    if (!selectedSubject) {
      return
    }
    appStoreActions.prepareUsageForSubject(selectedSubject, meterSummaries[0]?.meter ?? '')
    void router.navigate({ to: '/usage' })
  }

  async function exportSubjectEvents() {
    await appStoreActions.exportSelectedSubjectEvents()
  }

  return (
    <>
      <PageHeader
        eyebrow="Subjects"
        icon={<Users />}
        title="Subject activity"
        description="Inspect customer or account usage across meters and recent events."
        action={null}
      />

      {error ? <div className="error-banner">{error}</div> : null}

      <section className="mb-4 grid gap-4 md:grid-cols-3" aria-label="Subject metrics">
        <MetricCard icon={<Users />} label="Subjects" loading={metricsLoading} value={items.length} helper="Subjects with usage" />
        <MetricCard icon={<Hash />} label="Usage Events" loading={metricsLoading} value={sumSubjects(items, 'usage_events')} helper="Events in subject index" />
        <MetricCard icon={<Database />} label="Meter Links" loading={metricsLoading} value={sumSubjects(items, 'meters')} helper="Distinct subject meters" />
      </section>

      <section className="grid gap-4 xl:grid-cols-[minmax(280px,360px)_minmax(0,1fr)]">
        <Card className="min-w-0">
          <CardHeader className="!px-4 !py-3">
            <div>
              <CardTitle>Subjects</CardTitle>
              <CardDescription>Recent activity ordered by last event.</CardDescription>
            </div>
          </CardHeader>
          <CardContent>
            <div className="border-b border-border p-3">
              <Label className="grid gap-1.5 text-xs font-bold text-muted">
                Search
                <span className="relative block">
                  <Search className="pointer-events-none absolute left-3 top-1/2 size-4 -translate-y-1/2 text-muted" aria-hidden="true" />
                  <Input
                    aria-label="Search subjects"
                    className="h-10 bg-white pl-9 text-sm"
                    onChange={(event) => appStoreActions.setSubjectSearchQuery(event.currentTarget.value)}
                    placeholder="Search subjects"
                    value={searchQuery}
                  />
                </span>
              </Label>
            </div>
            <DataTable
              className="!min-w-0"
              emptyLabel={status === 'loading' ? 'Loading subjects' : 'No subjects found'}
              headers={['Subject']}
              rows={visibleSubjects.map((subject) => [
                <SubjectSelectButton
                  active={subject.subject === selectedSubject}
                  onSelect={() => selectSubject(subject.subject)}
                  subject={subject.subject}
                />,
              ])}
            />
            {nextCursor ? (
              <div className="pagination-actions">
                <Button disabled={loadingMore} onClick={() => void appStoreActions.loadMoreSubjects()} type="button" variant="outline">
                  {loadingMore ? <Loader2 className="spin" aria-hidden="true" /> : null}
                  Load more subjects
                </Button>
              </div>
            ) : null}
          </CardContent>
        </Card>

        <Card className="min-w-0">
          <CardHeader className="!px-4 !py-3">
            <div>
              <CardTitle>{selectedSubject || 'No subject selected'}</CardTitle>
              <CardDescription>{detailDescription(selectedStats, detailStatus, selectedSubject)}</CardDescription>
            </div>
            <div className="subject-detail-actions">
              <Button disabled={!selectedSubject || detailStatus === 'loading'} onClick={openUsageForSubject} type="button" variant="outline">
                <BarChart3 aria-hidden="true" />
                Open Usage
              </Button>
              <Button disabled={!selectedSubject || exporting} onClick={() => void exportSubjectEvents()} type="button" variant="outline">
                {exporting ? <Loader2 className="spin" aria-hidden="true" /> : <Download aria-hidden="true" />}
                Export Events
              </Button>
              <Badge variant={detailStatus === 'loading' ? 'muted' : selectedSubject ? 'success' : 'muted'}>
                {detailStatus === 'loading' ? 'Loading' : selectedSubject ? 'Selected' : 'Idle'}
              </Badge>
            </div>
          </CardHeader>
          <CardContent className="subject-detail-content">
            {exportError ? <div className="inline-error">{exportError}</div> : null}
            {selectedStats ? (
              <div className="subject-snapshot" aria-label="Selected subject summary">
                <SnapshotMetric icon={<BarChart3 />} label="Events" value={formatNumber(selectedStats.usage_events)} />
                <SnapshotMetric icon={<Database />} label="Meters" value={formatNumber(selectedStats.meters)} />
                <SnapshotMetric icon={<Clock />} label="Last Event" value={formatDate(selectedStats.last_event_at)} />
              </div>
            ) : (
              <p className="subject-empty">Select a subject to inspect recent activity.</p>
            )}

            <section className="subject-meter-section" aria-label="Subject meter activity">
              <div className="subject-section-heading">
                <h2>Meter Activity</h2>
                <span>{meterSummaries.length} meters</span>
              </div>
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
            </section>
          </CardContent>
        </Card>

        <Card className="min-w-0 xl:col-span-2 subject-events-card">
          <CardHeader className="!px-4 !py-3">
            <div>
              <CardTitle>Recent Events</CardTitle>
              <CardDescription>{selectedSubject ? `Latest usage for ${selectedSubject}` : 'Latest usage for the selected subject.'}</CardDescription>
            </div>
          </CardHeader>
          <CardContent>
            <DataTable
              emptyLabel={selectedSubject ? 'No recent events for this subject' : 'Select a subject to view events'}
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
      </section>
    </>
  )
}

function SubjectSelectButton({ active, onSelect, subject }: { active: boolean; onSelect: () => void; subject: string }) {
  return (
    <Button
      aria-pressed={active}
      className="subject-select-button"
      onClick={onSelect}
      type="button"
      variant={active ? 'secondary' : 'ghost'}
    >
      <span className="mono strong">{subject}</span>
    </Button>
  )
}

function SnapshotMetric({ icon, label, value }: { icon: React.ReactNode; label: string; value: string }) {
  return (
    <div className="subject-snapshot-item">
      <span>{icon}</span>
      <div>
        <small>{label}</small>
        <strong>{value}</strong>
      </div>
    </div>
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

function filterSubjects(subjects: SubjectStats[], searchQuery: string) {
  const query = searchQuery.trim().toLowerCase()
  if (!query) {
    return subjects
  }
  return subjects.filter((subject) => subject.subject.toLowerCase().includes(query))
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

function sumSubjects(subjects: SubjectStats[], field: 'meters' | 'usage_events') {
  return subjects.reduce((sum, subject) => sum + subject[field], 0)
}

function detailDescription(subject: SubjectStats | null, status: string, selectedSubject: string) {
  if (status === 'loading') {
    return 'Loading recent activity.'
  }
  if (!selectedSubject) {
    return 'Choose a subject from the list.'
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
