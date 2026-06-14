import { useSelector } from '@tanstack/react-store'
import { BarChart3, Loader2, RefreshCw, Search } from 'lucide-react'
import { type FormEvent, useCallback, useMemo } from 'react'

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
  selectedMeterSchemaKeys,
} from '../lib/usage-query'

export function UsagePage() {
  const { buckets, error, filterQuery, groupBy, meters, status } = useSelector(appStore, (state) => state.usage)
  const load = useCallback(() => appStoreActions.loadUsageControls(), [])

  useInitialLoad(load)

  async function submitQuery(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    const form = new FormData(event.currentTarget)
    await appStoreActions.submitUsageQuery(String(form.get('group_by') || ''), Number(form.get('limit') || 500), String(form.get('bucket_size') || 'day'))
  }

  const selectedMeterName = firstEqualRuleValue(filterQuery, 'meter')
  const groupKeys = useMemo(() => selectedMeterSchemaKeys(meters, selectedMeterName), [meters, selectedMeterName])
  const activeGroupBy = groupKeys.includes(groupBy) ? groupBy : ''
  const filterFields = useMemo(() => buildFilterFields(groupKeys, meters), [groupKeys, meters])

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
                <label>
                  Group By
                  <select aria-label="Group By" name="group_by" value={activeGroupBy} onChange={(event) => appStoreActions.setUsageGroupBy(event.target.value)}>
                    <option value="">None</option>
                    {groupKeys.map((key) => <option key={key} value={key}>{key}</option>)}
                  </select>
                </label>
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
                <span className="mono strong">{bucket.subject}</span>,
                bucket.meter,
                <Badge variant="muted">{bucket.aggregation}</Badge>,
                bucket.unit,
                <span className="mono truncate">{JSON.stringify(bucket.group || {})}</span>,
                formatNumber(bucket.quantity),
              ])}
            />
          </CardContent>
        </Card>
      </section>
    </>
  )
}
