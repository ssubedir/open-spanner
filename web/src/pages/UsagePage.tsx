import { useSelector } from '@tanstack/react-store'
import { BarChart3, Download, Loader2, Pin, PinOff, RefreshCw, Save, Search, Trash2 } from 'lucide-react'
import { type FormEvent, useCallback, useEffect, useMemo } from 'react'

import { appStore, appStoreActions } from '../app-store'
import { DataTable, Modal, PageHeader } from '../components/dashboard'
import { FilterBuilder } from '../components/filter-builder'
import { Badge } from '../components/ui/badge'
import { Button } from '../components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '../components/ui/card'
import { formatDate, formatNumber } from '../lib/format'
import { useInitialLoad } from '../lib/hooks'
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

export function UsagePage() {
  const {
    bucketSize,
    breakdownError,
    breakdowns,
    breakdownStatus,
    buckets,
    dimensionValues,
    error,
    exportError,
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

  useEffect(() => {
    void appStoreActions.loadUsageDimensionValues()
  }, [discoveryKey])

  useEffect(() => {
    void appStoreActions.loadUsageBreakdowns()
  }, [breakdownKey])

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

      <section className="usage-grid">
        <Card>
          <CardHeader>
            <div>
              <CardTitle>Usage Query</CardTitle>
              <CardDescription>Filter with rules, then choose the result shape.</CardDescription>
            </div>
          </CardHeader>
          <CardContent className="form-card">
            <form className="form-grid usage-query-form" onSubmit={(event) => void submitQuery(event)}>
              <div className="saved-query-controls wide">
                <label>
                  Saved Query
                  <select
                    aria-label="Saved query"
                    disabled={savedQueryStatus === 'loading'}
                    onChange={(event) => appStoreActions.selectSavedUsageQuery(event.target.value)}
                    value={selectedSavedQueryID}
                  >
                    <option value="">New query</option>
                    {savedQueries.map((query) => (
                      <option key={query.id} value={query.id}>{query.name}</option>
                    ))}
                  </select>
                </label>
                <label>
                  Name
                  <input
                    aria-label="Saved query name"
                    maxLength={120}
                    onChange={(event) => appStoreActions.setSavedUsageQueryName(event.target.value)}
                    placeholder="API usage by endpoint"
                    value={savedQueryName}
                  />
                </label>
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
                    onClick={() => selectedSavedQuery && void appStoreActions.toggleSavedUsageQueryPinned(selectedSavedQuery)}
                    type="button"
                    variant="outline"
                  >
                    {selectedSavedQuery?.pinned ? <PinOff aria-hidden="true" /> : <Pin aria-hidden="true" />}
                    {selectedSavedQuery?.pinned ? 'Unpin' : 'Pin'}
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
              <FilterBuilder
                fields={filterFields}
                metadataTypes={metadataTypes}
                onChange={appStoreActions.setUsageFilterQuery}
                query={filterQuery}
              />
              <div className="query-controls wide">
                <label>
                  Bucket
                  <select
                    aria-label="Bucket"
                    name="bucket_size"
                    onChange={(event) => appStoreActions.setUsageBucketSize(event.target.value)}
                    value={bucketSize}
                  >
                    <option value="day">Day</option>
                    <option value="hour">Hour</option>
                    <option value="month">Month</option>
                  </select>
                </label>
                <div className="dimension-picker">
                  <span>Group By</span>
                  <div className="dimension-options">
                    {groupKeys.map((key) => {
                      const active = activeGroupBy.includes(key)
                      return (
                        <label className="dimension-option" key={key}>
                          <input
                            checked={active}
                            disabled={!active && activeGroupBy.length >= maxGroupByFields}
                            onChange={() => appStoreActions.toggleUsageGroupBy(key)}
                            type="checkbox"
                          />
                          <span>{groupLabel(key, metadataLabels)}</span>
                        </label>
                      )
                    })}
                  </div>
                </div>
                <label>
                  Limit
                  <input
                    max="1000"
                    min="1"
                    name="limit"
                    onChange={(event) => appStoreActions.setUsageLimit(Number(event.target.value || 500))}
                    type="number"
                    value={limit}
                  />
                </label>
                <div className="query-actions">
                  <Button onClick={resetQuery} type="button" variant="outline">
                    <RefreshCw aria-hidden="true" />
                    Reset
                  </Button>
                  <Button disabled={exporting} onClick={() => void exportBuckets()} type="button" variant="outline">
                    {exporting ? <Loader2 className="spin" aria-hidden="true" /> : <Download aria-hidden="true" />}
                    Export CSV
                  </Button>
                  <Button disabled={status === 'loading'} type="submit">
                    {status === 'loading' ? <Loader2 className="spin" aria-hidden="true" /> : <Search aria-hidden="true" />}
                    Run Query
                  </Button>
                </div>
              </div>
              {exportError ? <div className="inline-error wide">{exportError}</div> : null}
            </form>
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
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

        <Card className="usage-results-card">
          <CardHeader>
            <div>
              <CardTitle>Results</CardTitle>
              <CardDescription>Bucketed usage returned by the current query.</CardDescription>
            </div>
            <Badge variant={buckets.length > 0 ? 'success' : 'muted'}>{buckets.length} rows</Badge>
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
    </>
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

function humanizeField(key: string) {
  return key
    .replace(/^metadata\./, '')
    .split(/[._-]/)
    .filter(Boolean)
    .map((part) => part.charAt(0).toUpperCase() + part.slice(1))
    .join(' ')
}
