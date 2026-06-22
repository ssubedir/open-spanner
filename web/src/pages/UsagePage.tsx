import { useSelector } from '@tanstack/react-store'
import { BarChart3, Copy, Download, FileClock, List, Loader2, RefreshCw, Save, Search, Trash2, X } from 'lucide-react'
import { type FormEvent, useCallback, useEffect, useMemo, useState } from 'react'

import { appStore, appStoreActions } from '../app-store'
import type { UsageEvent } from '../api'
import { DataTable, Modal, PageHeader } from '../components/dashboard'
import { ExportJobsCard } from '../components/export-jobs-card'
import { FilterBuilder } from '../components/filter-builder'
import { Badge } from '../components/ui/badge'
import { Button } from '../components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '../components/ui/card'
import { Checkbox } from '../components/ui/checkbox'
import { Input } from '../components/ui/input'
import { Label } from '../components/ui/label'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '../components/ui/select'
import { isActiveExportJob } from '../lib/export-jobs'
import { formatDate, formatNumber } from '../lib/format'
import { useInitialLoad } from '../lib/hooks'
import { UsageChart, type UsageChartMode } from '../components/usage-chart'
import {
  buildFilterFields,
  firstEqualRuleValue,
  metadataLabelsByField,
  metadataTypesByField,
  selectedMeterSchemaKeys,
  usageBreakdownQueryKey,
  usageDimensionDiscoveryKey,
} from '../lib/usage-query'

const maxGroupByFields = 5
const newSavedQuerySelectValue = '__new_query__'

export function UsagePage() {
  const [chartMode, setChartMode] = useState<UsageChartMode>('line')
  const [chartCumulative, setChartCumulative] = useState(false)
  const [chartShowPoints, setChartShowPoints] = useState(true)
  const [chartStacked, setChartStacked] = useState(false)
  const {
    bucketSize,
    breakdownError,
    breakdowns,
    breakdownStatus,
    buckets,
    dimensionValues,
    error,
    events,
    eventsError,
    eventsStatus,
    exportError,
    exportJobDownloading,
    exportJobError,
    exportJobLoadingMore,
    exportJobMutating,
    exportJobNextCursor,
    exportJobStatus,
    exportJobs,
    exporting,
    filterQuery,
    groupBy,
    limit,
    meters,
    savedQueryDeleting,
    savedQueryError,
    savedQueryName,
    savedQuerySaving,
    savedQueryStatus,
    savedQueries,
    selectedSavedQueryID,
    selectedUsageEvent,
    status,
  } = useSelector(appStore, (state) => state.usage)
  const load = useCallback(() => appStoreActions.loadUsageControls(), [])

  useInitialLoad(load)

  async function submitQuery(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    await appStoreActions.submitUsageQuery(activeGroupBy, limit, bucketSize)
  }

  async function saveQuery() {
    await appStoreActions.saveCurrentUsageQuery()
  }

  async function exportBuckets() {
    await appStoreActions.exportCurrentUsageBuckets(activeGroupBy, limit, bucketSize)
  }

  async function exportEvents() {
    await appStoreActions.exportCurrentUsageEvents(limit)
  }

  async function viewEvents() {
    await appStoreActions.loadCurrentUsageEvents(limit)
  }

  async function queueExport() {
    await appStoreActions.queueCurrentUsageExport(activeGroupBy, limit, bucketSize)
  }

  async function changeChartBucketSize(nextBucketSize: string) {
    appStoreActions.setUsageBucketSize(nextBucketSize)
    if (buckets.length > 0) {
      await appStoreActions.submitUsageQuery(activeGroupBy, limit, nextBucketSize)
    }
  }

  async function confirmDeleteSavedQuery() {
    await appStoreActions.deleteSelectedSavedUsageQuery()
  }

  const selectedMeterName = firstEqualRuleValue(filterQuery, 'meter')
  const metadataKeys = useMemo(() => selectedMeterSchemaKeys(meters, selectedMeterName), [meters, selectedMeterName])
  const groupKeys = useMemo(() => ['subject', ...metadataKeys], [metadataKeys])
  const breakdownFields = useMemo(() => ['subject', ...metadataKeys].slice(0, 5), [metadataKeys])
  const activeGroupBy = groupBy.filter((key) => groupKeys.includes(key))
  const metadataLabels = useMemo(() => metadataLabelsByField(meters, selectedMeterName), [meters, selectedMeterName])
  const metadataTypes = useMemo(() => metadataTypesByField(meters, selectedMeterName), [meters, selectedMeterName])
  const filterFields = useMemo(
    () => buildFilterFields(metadataKeys, meters, dimensionValues, metadataTypes, metadataLabels),
    [dimensionValues, metadataKeys, metadataLabels, metadataTypes, meters],
  )
  const discoveryKey = useMemo(() => usageDimensionDiscoveryKey(filterQuery, meters), [filterQuery, meters])
  const breakdownKey = useMemo(() => usageBreakdownQueryKey(filterQuery, meters), [filterQuery, meters])
  const breakdownSections = useMemo(
    () => breakdownFields.map((field) => ({ field, items: breakdowns[field] || [] })),
    [breakdownFields, breakdowns],
  )
  const selectedSavedQuery = savedQueries.find((item) => item.id === selectedSavedQueryID)
  const exportInProgress = exporting !== ''
  const hasActiveExportJobs = useMemo(() => exportJobs.some(isActiveExportJob), [exportJobs])
  const canStackChart = activeGroupBy.length > 0
  const chartControls = useMemo(
    () => ({
      cumulative: chartCumulative,
      mode: chartMode,
      showPoints: chartShowPoints,
      stacked: canStackChart && chartStacked,
    }),
    [canStackChart, chartCumulative, chartMode, chartShowPoints, chartStacked],
  )

  useEffect(() => {
    void appStoreActions.loadUsageDimensionValues()
  }, [discoveryKey])

  useEffect(() => {
    void appStoreActions.loadUsageBreakdowns()
  }, [breakdownKey])

  useEffect(() => {
    if (!hasActiveExportJobs) {
      return
    }

    const poll = window.setInterval(() => {
      void appStoreActions.loadUsageExportJobs(undefined, { preserveLoaded: true, quiet: true })
    }, 5000)
    return () => window.clearInterval(poll)
  }, [hasActiveExportJobs])

  function resetQuery() {
    appStoreActions.resetUsageQuery()
  }

  return (
    <>
      <PageHeader
        eyebrow="Usage"
        icon={<BarChart3 />}
        title="Usage buckets"
        description="Query bucketed usage with a time window, bucket settings, and advanced filters."
        action={null}
      />

      {error ? <div className="error-banner">{error}</div> : null}

      <section className="grid gap-3">
        <Card className="usage-query-card">
          <CardHeader className="!px-4 !py-3">
            <div>
              <CardTitle>Usage Query</CardTitle>
              <CardDescription>Filter with rules, then choose the result shape.</CardDescription>
            </div>
          </CardHeader>
          <CardContent className="usage-query-content">
            <form className="usage-query-form" onSubmit={(event) => void submitQuery(event)}>
              <div className="saved-query-controls wide">
                <Label className="grid gap-1.5">
                  Saved Query
                  <Select
                    disabled={savedQueryStatus === 'loading'}
                    onValueChange={(value) => appStoreActions.selectSavedUsageQuery(value === newSavedQuerySelectValue ? '' : value)}
                    value={selectedSavedQueryID || newSavedQuerySelectValue}
                  >
                    <SelectTrigger aria-label="Saved query" className="min-h-[38px] w-full">
                      <SelectValue placeholder="Select saved query" />
                    </SelectTrigger>
                    <SelectContent position="popper">
                      <SelectItem value={newSavedQuerySelectValue}>New query</SelectItem>
                      {savedQueries.map((query) => (
                        <SelectItem key={query.id} value={query.id}>{query.name}</SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                </Label>
                <Label className="grid gap-1.5">
                  Name
                  <Input
                    aria-label="Saved query name"
                    maxLength={120}
                    onChange={(event) => appStoreActions.setSavedUsageQueryName(event.target.value)}
                    placeholder="API usage by endpoint"
                    value={savedQueryName}
                  />
                </Label>
                <div className="saved-query-actions">
                  <Button
                    disabled={savedQuerySaving || savedQueryName.trim() === ''}
                    onClick={() => void saveQuery()}
                    type="button"
                  >
                    {savedQuerySaving ? <Loader2 className="spin" aria-hidden="true" /> : <Save aria-hidden="true" />}
                    {selectedSavedQueryID ? 'Update' : 'Save'}
                  </Button>
                  <Button
                    disabled={!selectedSavedQuery || savedQuerySaving}
                    onClick={() => selectedSavedQuery && appStoreActions.setSavedUsageQueryDeleting(selectedSavedQuery)}
                    type="button"
                    variant="outline"
                  >
                    <Trash2 aria-hidden="true" />
                    Delete
                  </Button>
                </div>
              </div>
              {savedQueryError ? <div className="inline-error wide">{savedQueryError}</div> : null}
              <div className="usage-query-layout wide">
                <FilterBuilder
                  fields={filterFields}
                  metadataTypes={metadataTypes}
                  onChange={appStoreActions.setUsageFilterQuery}
                  query={filterQuery}
                />
                <div className="query-controls">
                  <div className="query-control-heading">
                    <span>Result Shape</span>
                    <small>{activeGroupBy.length}/{maxGroupByFields} groups</small>
                  </div>
                  <Label className="grid gap-1.5">
                    Bucket
                    <Select
                      name="bucket_size"
                      onValueChange={appStoreActions.setUsageBucketSize}
                      value={bucketSize}
                    >
                      <SelectTrigger aria-label="Bucket" className="min-h-[38px] w-full">
                        <SelectValue placeholder="Select bucket" />
                      </SelectTrigger>
                      <SelectContent position="popper">
                        <SelectItem value="day">Day</SelectItem>
                        <SelectItem value="hour">Hour</SelectItem>
                        <SelectItem value="month">Month</SelectItem>
                      </SelectContent>
                    </Select>
                  </Label>
                  <div className="dimension-picker">
                    <span>Group By</span>
                    <div className="dimension-options">
                      {groupKeys.map((key) => {
                        const active = activeGroupBy.includes(key)
                        return (
                          <Label className="dimension-option" key={key}>
                            <Checkbox
                              checked={active}
                              disabled={!active && activeGroupBy.length >= maxGroupByFields}
                              onCheckedChange={() => appStoreActions.toggleUsageGroupBy(key)}
                            />
                            <span>{groupLabel(key, metadataLabels)}</span>
                          </Label>
                        )
                      })}
                    </div>
                  </div>
                  <Label className="grid gap-1.5">
                    Limit
                    <Input
                      max="1000"
                      min="1"
                      name="limit"
                      onChange={(event) => appStoreActions.setUsageLimit(Number(event.target.value || 500))}
                      type="number"
                      value={limit}
                    />
                  </Label>
                  <div className="query-actions">
                    <Button onClick={resetQuery} type="button" variant="outline">
                      <RefreshCw aria-hidden="true" />
                      Reset
                    </Button>
                    <Button disabled={exportInProgress} onClick={() => void exportBuckets()} type="button" variant="outline">
                      {exporting === 'buckets' ? <Loader2 className="spin" aria-hidden="true" /> : <Download aria-hidden="true" />}
                      Export Buckets
                    </Button>
                    <Button disabled={exportInProgress} onClick={() => void exportEvents()} type="button" variant="outline">
                      {exporting === 'events' ? <Loader2 className="spin" aria-hidden="true" /> : <Download aria-hidden="true" />}
                      Export Events
                    </Button>
                    <Button disabled={exportInProgress} onClick={() => void queueExport()} type="button" variant="outline">
                      {exporting === 'job' ? <Loader2 className="spin" aria-hidden="true" /> : <FileClock aria-hidden="true" />}
                      Queue Export
                    </Button>
                    <Button disabled={eventsStatus === 'loading'} onClick={() => void viewEvents()} type="button" variant="outline">
                      {eventsStatus === 'loading' ? <Loader2 className="spin" aria-hidden="true" /> : <List aria-hidden="true" />}
                      View Events
                    </Button>
                    <Button disabled={status === 'loading'} type="submit">
                      {status === 'loading' ? <Loader2 className="spin" aria-hidden="true" /> : <Search aria-hidden="true" />}
                      Run Query
                    </Button>
                  </div>
                </div>
              </div>
              {exportError ? <div className="inline-error wide">{exportError}</div> : null}
            </form>
          </CardContent>
        </Card>

        <ExportJobsCard
          downloadingID={exportJobDownloading}
          error={exportJobError}
          hasMore={Boolean(exportJobNextCursor)}
          jobs={exportJobs}
          loadingMore={exportJobLoadingMore}
          mutatingID={exportJobMutating}
          onLoadMore={() => void appStoreActions.loadMoreUsageExportJobs()}
          status={exportJobStatus}
        />

        <Card className="usage-breakdown-card">
          <CardHeader className="!px-4 !py-3">
            <div>
              <CardTitle>Breakdowns</CardTitle>
              <CardDescription>Top subjects and dimensions for the current query window.</CardDescription>
            </div>
            <Badge variant={breakdownStatus === 'error' ? 'warning' : breakdownStatus === 'loading' ? 'muted' : 'success'}>
              {breakdownStatus === 'loading' ? 'Loading' : `${breakdownSections.length} fields`}
            </Badge>
          </CardHeader>
          <CardContent className="breakdown-content">
            {breakdownError ? <div className="inline-error">{breakdownError}</div> : null}
            {breakdownSections.length > 0 ? (
              <div className="breakdown-grid">
                {breakdownSections.map((section) => (
                  <BreakdownPanel
                    field={section.field}
                    items={section.items}
                    label={breakdownLabel(section.field, metadataLabels)}
                    key={section.field}
                    onApplyFilter={appStoreActions.applyUsageBreakdownFilter}
                  />
                ))}
              </div>
            ) : (
              <div className="breakdown-empty">Choose a meter and time range to view breakdowns.</div>
            )}
          </CardContent>
        </Card>

        <Card className="usage-chart-card">
          <CardHeader className="usage-chart-header">
            <div>
              <CardTitle>Usage Over Time</CardTitle>
              <CardDescription>Graph the current query by bucket, chart type, and series behavior.</CardDescription>
            </div>
            <div className="usage-chart-controls" aria-label="Usage chart controls">
              <Label className="grid gap-1.5">
                Chart Bucket
                <Select
                  disabled={status === 'loading'}
                  onValueChange={(value) => void changeChartBucketSize(value)}
                  value={bucketSize}
                >
                  <SelectTrigger aria-label="Chart bucket" className="min-h-[38px] w-full">
                    <SelectValue placeholder="Select chart bucket" />
                  </SelectTrigger>
                  <SelectContent position="popper">
                    <SelectItem value="hour">Hour</SelectItem>
                    <SelectItem value="day">Day</SelectItem>
                    <SelectItem value="month">Month</SelectItem>
                  </SelectContent>
                </Select>
              </Label>
              <Label className="grid gap-1.5">
                Chart Type
                <Select
                  onValueChange={(value) => setChartMode(value as UsageChartMode)}
                  value={chartMode}
                >
                  <SelectTrigger aria-label="Chart type" className="min-h-[38px] w-full">
                    <SelectValue placeholder="Select chart type" />
                  </SelectTrigger>
                  <SelectContent position="popper">
                    <SelectItem value="line">Line</SelectItem>
                    <SelectItem value="area">Filled Area</SelectItem>
                    <SelectItem value="bar">Bar</SelectItem>
                  </SelectContent>
                </Select>
              </Label>
              <Label
                className={`checkbox-row usage-chart-toggle${canStackChart ? '' : ' disabled'}`}
                title={canStackChart ? 'Stack grouped chart series.' : 'Choose a Group By field to enable stacking.'}
              >
                <Checkbox
                  aria-label="Stack chart series"
                  checked={canStackChart && chartStacked}
                  disabled={!canStackChart}
                  onCheckedChange={(checked) => setChartStacked(checked === true)}
                />
                <span>Stack</span>
              </Label>
              <Label className="checkbox-row usage-chart-toggle">
                <Checkbox
                  aria-label="Cumulative chart"
                  checked={chartCumulative}
                  onCheckedChange={(checked) => setChartCumulative(checked === true)}
                />
                <span>Cumulative</span>
              </Label>
              <Label className="checkbox-row usage-chart-toggle">
                <Checkbox
                  aria-label="Show chart points"
                  checked={chartShowPoints}
                  onCheckedChange={(checked) => setChartShowPoints(checked === true)}
                />
                <span>Points</span>
              </Label>
            </div>
          </CardHeader>
          <CardContent>
            <UsageChart bucketSize={bucketSize} buckets={buckets} controls={chartControls} groupBy={activeGroupBy} />
          </CardContent>
        </Card>

        <Card className="usage-results-card">
          <CardHeader>
            <div>
              <CardTitle>Results</CardTitle>
              <CardDescription>Bucketed usage returned by the current query.</CardDescription>
            </div>
          </CardHeader>
          <CardContent>
            <DataTable
              emptyLabel="Run a query to view usage"
              headers={['Bucket Start', 'Subject', 'Meter', 'Aggregation', 'Unit', 'Group', 'Quantity']}
              rows={buckets.map((bucket) => [
                formatDate(bucket.bucket_start),
                <SubjectValue subject={bucket.subject} />,
                bucket.meter,
                <Badge variant="muted">{bucket.aggregation}</Badge>,
                bucket.unit,
                <GroupValues group={bucket.group} />,
                formatNumber(bucket.quantity),
              ])}
            />
          </CardContent>
        </Card>

        <UsageEventsCard
          error={eventsError}
          events={events}
          onSelectEvent={appStoreActions.setSelectedUsageEvent}
          status={eventsStatus}
        />
      </section>

      {savedQueryDeleting ? (
        <Modal title="Delete Saved Query" onClose={() => appStoreActions.setSavedUsageQueryDeleting(null)}>
          <div className="modal-copy">Delete <strong>{savedQueryDeleting.name}</strong>?</div>
          <div className="modal-actions">
            <Button onClick={() => appStoreActions.setSavedUsageQueryDeleting(null)} type="button" variant="outline">Cancel</Button>
            <Button disabled={savedQuerySaving} onClick={() => void confirmDeleteSavedQuery()} type="button">Delete</Button>
          </div>
        </Modal>
      ) : null}

      {selectedUsageEvent ? (
        <UsageEventDrawer
          event={selectedUsageEvent}
          onClose={() => appStoreActions.setSelectedUsageEvent(null)}
        />
      ) : null}
    </>
  )
}

function UsageEventsCard({
  error,
  events,
  onSelectEvent,
  status,
}: {
  error: string
  events: UsageEvent[]
  onSelectEvent: (event: UsageEvent) => void
  status: string
}) {
  return (
    <Card className="usage-events-card">
      <CardHeader>
        <div>
          <CardTitle>Events</CardTitle>
          <CardDescription>Raw usage events matching the current filter.</CardDescription>
        </div>
      </CardHeader>
      <CardContent>
        {error ? <div className="inline-error">{error}</div> : null}
        <DataTable
          emptyLabel={status === 'loading' ? 'Loading events' : 'View events to inspect raw usage'}
          headers={['Timestamp', 'Subject', 'Meter', 'Quantity', 'Metadata', 'Idempotency Key', 'Event ID', 'Details']}
          rows={events.map((event) => [
            formatDate(event.timestamp),
            <SubjectValue subject={event.subject} />,
            <Badge variant="muted">{event.meter}</Badge>,
            formatNumber(event.quantity),
            <MetadataValues metadata={event.metadata} />,
            event.idempotency_key ? <span className="mono truncate">{event.idempotency_key}</span> : <span className="muted">none</span>,
            <span className="mono truncate">{event.id}</span>,
            <Button
              aria-label={`View details for event ${event.id}`}
              onClick={() => onSelectEvent(event)}
              size="sm"
              type="button"
              variant="outline"
            >
              Details
            </Button>,
          ])}
        />
      </CardContent>
    </Card>
  )
}

function UsageEventDrawer({
  event,
  onClose,
}: {
  event: UsageEvent
  onClose: () => void
}) {
  const metadataJSON = JSON.stringify(event.metadata || {}, null, 2)

  return (
    <div
      className="usage-event-drawer-backdrop"
      onMouseDown={(mouseEvent) => {
        if (mouseEvent.target === mouseEvent.currentTarget) {
          onClose()
        }
      }}
      role="presentation"
    >
      <aside
        aria-labelledby="usage-event-detail-title"
        aria-modal="true"
        className="usage-event-drawer"
        role="dialog"
      >
        <header className="usage-event-drawer-header">
          <div>
            <span>Usage Event</span>
            <h2 id="usage-event-detail-title">{event.subject}</h2>
          </div>
          <Button aria-label="Close event details" onClick={onClose} size="icon" type="button" variant="ghost">
            <X aria-hidden="true" />
          </Button>
        </header>

        <div className="usage-event-detail-grid">
          <EventDetailItem copyLabel="Copy event ID" label="Event ID" value={event.id} />
          <EventDetailItem copyLabel="Copy idempotency key" label="Idempotency Key" value={event.idempotency_key || 'none'} />
          <EventDetailItem copyLabel="Copy subject" label="Subject" value={event.subject} />
          <EventDetailItem copyLabel="Copy meter" label="Meter" value={event.meter} />
          <EventDetailItem label="Quantity" value={formatNumber(event.quantity)} />
          <EventDetailItem copyLabel="Copy timestamp" label="Timestamp" value={event.timestamp} />
          <EventDetailItem copyLabel="Copy received time" label="Received At" value={event.received_at} />
        </div>

        <section className="usage-event-metadata-panel">
          <div className="usage-event-section-header">
            <h3>Metadata</h3>
            <Button aria-label="Copy metadata" onClick={() => void copyText(metadataJSON)} size="sm" type="button" variant="outline">
              <Copy aria-hidden="true" />
              Copy
            </Button>
          </div>
          <pre>{metadataJSON}</pre>
        </section>
      </aside>
    </div>
  )
}

function EventDetailItem({
  copyLabel,
  label,
  value,
}: {
  copyLabel?: string
  label: string
  value: string
}) {
  return (
    <div className="usage-event-detail-item">
      <span>{label}</span>
      <div>
        <strong className="mono">{value}</strong>
        {copyLabel ? (
          <Button aria-label={copyLabel} onClick={() => void copyText(value)} size="icon" type="button" variant="ghost">
            <Copy aria-hidden="true" />
          </Button>
        ) : null}
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

function BreakdownPanel({
  field,
  items,
  label,
  onApplyFilter,
}: {
  field: string
  items: Array<{ value: string; quantity: number; events: number; unit: string }>
  label: string
  onApplyFilter: (field: string, value: string) => void
}) {
  const maxQuantity = Math.max(...items.map((item) => item.quantity), 0)

  return (
    <section className="breakdown-panel">
      <div className="breakdown-panel-header">
        <div>
          <h2>{label}</h2>
          <span>{items.length} values</span>
        </div>
      </div>
      {items.length > 0 ? (
        <div className="breakdown-list">
          {items.map((item, index) => (
            <button
              aria-label={`Filter by ${label}: ${item.value}`}
              className="breakdown-row"
              key={item.value}
              onClick={() => onApplyFilter(field, item.value)}
              title={`Filter by ${label}: ${item.value}`}
              type="button"
            >
              <span className="breakdown-rank">{index + 1}</span>
              <div className="breakdown-row-main">
                <div className="breakdown-label">
                  <strong>{item.value}</strong>
                  <small>{item.events} events</small>
                </div>
                <div className="breakdown-track">
                  <span style={{ width: `${breakdownWidth(item.quantity, maxQuantity)}%` }} />
                </div>
              </div>
              <div className="breakdown-value">
                <strong>{formatNumber(item.quantity)}</strong>
                <small>{item.unit}</small>
              </div>
            </button>
          ))}
        </div>
      ) : (
        <p className="breakdown-empty">No values found</p>
      )}
    </section>
  )
}

function SubjectValue({ subject }: { subject: string }) {
  if (!subject) {
    return <span className="muted">all subjects</span>
  }

  return <span className="mono strong">{subject}</span>
}

function GroupValues({ group }: { group?: Record<string, string> }) {
  const entries = Object.entries(group || {})
  if (entries.length === 0) {
    return <span className="muted">none</span>
  }

  return (
    <div className="dimension-values">
      {entries.map(([key, value]) => (
        <span className="dimension-value" key={key}>
          <span>{key}</span>
          <strong>{value}</strong>
        </span>
      ))}
    </div>
  )
}

function groupLabel(key: string, metadataLabels: Record<string, string> = {}) {
  return key === 'subject' ? 'Subject' : metadataLabels[`metadata.${key}`] || humanizeField(key)
}

function breakdownLabel(key: string, metadataLabels: Record<string, string> = {}) {
  return key === 'subject' ? 'Subjects' : metadataLabels[`metadata.${key}`] || humanizeField(key)
}

function breakdownWidth(quantity: number, maxQuantity: number) {
  if (maxQuantity <= 0) {
    return 4
  }
  return Math.max(4, (quantity / maxQuantity) * 100)
}

async function copyText(value: string) {
  if (!navigator.clipboard) {
    return
  }

  try {
    await navigator.clipboard.writeText(value)
  } catch {
    // Copying is a convenience action; failing silently keeps the drawer usable.
  }
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

function humanizeField(key: string) {
  return key
    .replace(/^metadata\./, '')
    .split(/[._-]/)
    .filter(Boolean)
    .map((part) => part.charAt(0).toUpperCase() + part.slice(1))
    .join(' ')
}
