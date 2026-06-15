import { useSelector } from '@tanstack/react-store'
import { BarChart3, Loader2, RefreshCw, Search } from 'lucide-react'
import { type FormEvent, useCallback, useEffect, useMemo } from 'react'

import { appStore, appStoreActions } from '../app-store'
import { DataTable, PageHeader } from '../components/dashboard'
import { FilterBuilder } from '../components/filter-builder'
import { Badge } from '../components/ui/badge'
import { Button } from '../components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '../components/ui/card'
import { formatDate, formatNumber } from '../lib/format'
import { useInitialLoad } from '../lib/hooks'
import {
  buildFilterFields,
  firstEqualRuleValue,
  metadataTypesByField,
  selectedMeterSchemaKeys,
  usageBreakdownQueryKey,
  usageDimensionDiscoveryKey,
} from '../lib/usage-query'

const maxGroupByFields = 5

export function UsagePage() {
  const {
    breakdownError,
    breakdowns,
    breakdownStatus,
    buckets,
    dimensionValues,
    error,
    filterQuery,
    groupBy,
    meters,
    status,
  } = useSelector(appStore, (state) => state.usage)
  const load = useCallback(() => appStoreActions.loadUsageControls(), [])

  useInitialLoad(load)

  async function submitQuery(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    const form = new FormData(event.currentTarget)
    await appStoreActions.submitUsageQuery(activeGroupBy, Number(form.get('limit') || 500), String(form.get('bucket_size') || 'day'))
  }

  const selectedMeterName = firstEqualRuleValue(filterQuery, 'meter')
  const metadataKeys = useMemo(() => selectedMeterSchemaKeys(meters, selectedMeterName), [meters, selectedMeterName])
  const groupKeys = useMemo(() => ['subject', ...metadataKeys], [metadataKeys])
  const breakdownFields = useMemo(() => ['subject', ...metadataKeys].slice(0, 5), [metadataKeys])
  const activeGroupBy = groupBy.filter((key) => groupKeys.includes(key))
  const metadataTypes = useMemo(() => metadataTypesByField(meters, selectedMeterName), [meters, selectedMeterName])
  const filterFields = useMemo(
    () => buildFilterFields(metadataKeys, meters, dimensionValues, metadataTypes),
    [dimensionValues, metadataKeys, metadataTypes, meters],
  )
  const discoveryKey = useMemo(() => usageDimensionDiscoveryKey(filterQuery, meters), [filterQuery, meters])
  const breakdownKey = useMemo(() => usageBreakdownQueryKey(filterQuery, meters), [filterQuery, meters])
  const breakdownSections = useMemo(
    () => breakdownFields.map((field) => ({ field, items: breakdowns[field] || [] })),
    [breakdownFields, breakdowns],
  )

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
              <FilterBuilder
                fields={filterFields}
                metadataTypes={metadataTypes}
                onChange={appStoreActions.setUsageFilterQuery}
                query={filterQuery}
              />
              <div className="query-controls wide">
                <label>
                  Bucket
                  <select aria-label="Bucket" name="bucket_size">
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
                          <span>{groupLabel(key)}</span>
                        </label>
                      )
                    })}
                  </div>
                </div>
                <label>
                  Limit
                  <input defaultValue="500" max="1000" min="1" name="limit" type="number" />
                </label>
                <div className="query-actions">
                  <Button onClick={resetQuery} type="button" variant="outline">
                    <RefreshCw aria-hidden="true" />
                    Reset
                  </Button>
                  <Button disabled={status === 'loading'} type="submit">
                    {status === 'loading' ? <Loader2 className="spin" aria-hidden="true" /> : <Search aria-hidden="true" />}
                    Run Query
                  </Button>
                </div>
              </div>
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
                  <BreakdownPanel field={section.field} items={section.items} key={section.field} />
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
    </>
  )
}

function BreakdownPanel({ field, items }: { field: string; items: Array<{ value: string; quantity: number; events: number; unit: string }> }) {
  const maxQuantity = Math.max(...items.map((item) => item.quantity), 0)

  return (
    <section className="breakdown-panel">
      <div className="breakdown-panel-header">
        <div>
          <h2>{breakdownLabel(field)}</h2>
          <span>{items.length} values</span>
        </div>
      </div>
      {items.length > 0 ? (
        <div className="breakdown-list">
          {items.map((item, index) => (
            <div className="breakdown-row" key={item.value}>
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
            </div>
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

function groupLabel(key: string) {
  return key === 'subject' ? 'Subject' : humanizeField(key)
}

function breakdownLabel(key: string) {
  return key === 'subject' ? 'Subjects' : humanizeField(key)
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
