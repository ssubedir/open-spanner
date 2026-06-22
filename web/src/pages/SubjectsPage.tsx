import { useRouter } from '@tanstack/react-router'
import { useSelector } from '@tanstack/react-store'
import { ArrowRight, Database, Hash, Loader2, Search, Users } from 'lucide-react'
import { useCallback, useMemo } from 'react'

import { appStore, appStoreActions } from '../app-store'
import type { SubjectStats } from '../api'
import { DataTable, MetricCard, PageHeader } from '../components/dashboard'
import { Button } from '../components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '../components/ui/card'
import { Input } from '../components/ui/input'
import { Label } from '../components/ui/label'
import { formatDate, formatNumber } from '../lib/format'
import { useInitialLoad } from '../lib/hooks'

export function SubjectsPage() {
  const router = useRouter()
  const {
    error,
    items,
    loadingMore,
    nextCursor,
    searchQuery,
    status,
  } = useSelector(appStore, (state) => state.subjects)
  const load = useCallback(() => appStoreActions.loadSubjects(), [])

  useInitialLoad(load)

  const visibleSubjects = useMemo(
    () => filterSubjects(items, searchQuery),
    [items, searchQuery],
  )
  const metricsLoading = status === 'idle' || (status === 'loading' && items.length === 0)

  function openSubject(subject: string) {
    void router.navigate({ to: '/subjects/$subject', params: { subject } })
  }

  return (
    <>
      <PageHeader
        eyebrow="Subjects"
        icon={<Users />}
        title="Subjects"
        description="Find customers and accounts with usage, then open one for activity and entitlement details."
        action={null}
      />

      {error ? <div className="error-banner">{error}</div> : null}

      <section className="mb-4 grid gap-4 md:grid-cols-3" aria-label="Subject metrics">
        <MetricCard icon={<Users />} label="Subjects" loading={metricsLoading} value={items.length} helper="Subjects with usage" />
        <MetricCard icon={<Hash />} label="Usage Events" loading={metricsLoading} value={sumSubjects(items, 'usage_events')} helper="Events in subject index" />
        <MetricCard icon={<Database />} label="Meter Links" loading={metricsLoading} value={sumSubjects(items, 'meters')} helper="Distinct subject meters" />
      </section>

      <Card className="max-w-[1480px] min-w-0">
        <CardHeader className="!px-4 !py-3">
          <div>
            <CardTitle>Subject directory</CardTitle>
            <CardDescription>Recent activity ordered by last event.</CardDescription>
          </div>
        </CardHeader>
        <CardContent>
          <div className="border-b border-border p-3">
            <Label className="grid max-w-[420px] gap-1.5 text-xs font-bold text-muted">
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
            headers={['Subject', 'Events', 'Meters', 'Last Event', 'Actions']}
            rows={visibleSubjects.map((subject) => [
              <span className="mono strong">{subject.subject}</span>,
              formatNumber(subject.usage_events),
              formatNumber(subject.meters),
              formatDate(subject.last_event_at),
              <span className="table-actions">
                <Button aria-label={`Open ${subject.subject}`} onClick={() => openSubject(subject.subject)} size="sm" type="button" variant="outline">
                  Open
                  <ArrowRight aria-hidden="true" />
                </Button>
              </span>,
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
    </>
  )
}

function filterSubjects(subjects: SubjectStats[], searchQuery: string) {
  const query = searchQuery.trim().toLowerCase()
  if (!query) {
    return subjects
  }
  return subjects.filter((subject) => subject.subject.toLowerCase().includes(query))
}

function sumSubjects(subjects: SubjectStats[], field: 'meters' | 'usage_events') {
  return subjects.reduce((sum, subject) => sum + subject[field], 0)
}
