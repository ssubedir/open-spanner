import {
  BarController,
  BarElement,
  CategoryScale,
  Chart as ChartJS,
  Filler,
  Legend,
  LinearScale,
  LineController,
  LineElement,
  PointElement,
  Tooltip,
  type ChartData,
  type ChartDataset,
  type ChartOptions,
} from 'chart.js'
import { Chart } from 'react-chartjs-2'
import { BarChart3 } from 'lucide-react'
import { useMemo } from 'react'

import type { UsageBucket } from '../api'
import { formatNumber } from '../lib/format'

ChartJS.register(
  BarController,
  BarElement,
  CategoryScale,
  LinearScale,
  LineController,
  LineElement,
  PointElement,
  Filler,
  Tooltip,
  Legend,
)

export type UsageChartMode = 'line' | 'area' | 'bar'

export type UsageChartControls = {
  cumulative: boolean
  mode: UsageChartMode
  showPoints: boolean
  stacked: boolean
}

const chartColors = [
  '#0f766e',
  '#2563eb',
  '#b45309',
  '#7c3aed',
  '#be123c',
  '#047857',
  '#c2410c',
  '#0369a1',
]

export function UsageChart({
  bucketSize,
  buckets,
  controls,
  groupBy,
}: {
  bucketSize: string
  buckets: UsageBucket[]
  controls: UsageChartControls
  groupBy: string[]
}) {
  const chartType = controls.mode === 'bar' ? 'bar' : 'line'
  const { data, seriesCount, total } = useMemo(
    () => usageChartData(buckets, groupBy, controls),
    [buckets, controls, groupBy],
  )
  const effectiveControls = useMemo(
    () => ({ ...controls, stacked: controls.stacked && seriesCount > 1 }),
    [controls, seriesCount],
  )
  const options = useMemo(() => usageChartOptions(bucketSize, effectiveControls), [bucketSize, effectiveControls])

  if (buckets.length === 0) {
    return (
      <div className="usage-chart-empty">
        <BarChart3 aria-hidden="true" />
        <span>Run a query to chart usage over time.</span>
      </div>
    )
  }

  return (
    <div className="usage-chart-shell">
      <div className="usage-chart-summary" aria-label="Usage chart summary">
        <span>{seriesCount} {seriesCount === 1 ? 'series' : 'series'}</span>
        <strong>{formatNumber(total)}</strong>
        <span>total units</span>
        {controls.cumulative ? <span>Cumulative</span> : null}
        {effectiveControls.stacked ? <span>Stacked</span> : null}
        {controls.stacked && !effectiveControls.stacked ? <span>Stack needs 2+ series</span> : null}
      </div>
      <div className="usage-chart-canvas">
        <Chart datasetIdKey="label" type={chartType} data={data} options={options} />
      </div>
    </div>
  )
}

function usageChartData(buckets: UsageBucket[], groupBy: string[], controls: UsageChartControls): {
  data: ChartData<'line' | 'bar', number[], string>
  seriesCount: number
  total: number
} {
  const labels = Array.from(new Set(buckets.map((bucket) => bucket.bucket_start))).sort()
  const totalsBySeries = new Map<string, Map<string, number>>()
  let total = 0

  for (const bucket of buckets) {
    const series = seriesLabel(bucket, groupBy)
    const values = totalsBySeries.get(series) ?? new Map<string, number>()
    values.set(bucket.bucket_start, (values.get(bucket.bucket_start) ?? 0) + bucket.quantity)
    totalsBySeries.set(series, values)
    total += bucket.quantity
  }

  const datasets = Array.from(totalsBySeries.entries())
    .sort(([left], [right]) => left.localeCompare(right))
    .map(([label, values], index) => {
      const color = chartColors[index % chartColors.length]
      const isArea = controls.mode === 'area'
      const isBar = controls.mode === 'bar'
      const rawValues = labels.map((bucketStart) => values.get(bucketStart) ?? 0)
      return {
        label,
        data: controls.cumulative ? cumulativeValues(rawValues) : rawValues,
        borderColor: color,
        backgroundColor: transparentColor(color, isBar ? 0.34 : isArea ? 0.26 : 0.1),
        borderWidth: isArea ? 1.5 : 2,
        borderRadius: isBar ? 4 : 0,
        fill: isArea ? true : false,
        pointRadius: !isArea && controls.showPoints && labels.length <= 48 ? 2.5 : 0,
        pointHoverRadius: isArea ? 3 : 4,
        stepped: isArea ? 'middle' : false,
        tension: isArea ? 0 : 0.28,
      } satisfies ChartDataset<'line' | 'bar', number[]>
    }) satisfies ChartDataset<'line' | 'bar', number[]>[]

  return {
    data: { labels, datasets },
    seriesCount: datasets.length,
    total,
  }
}

function usageChartOptions(bucketSize: string, controls: UsageChartControls): ChartOptions<'line' | 'bar'> {
  return {
    animation: false,
    maintainAspectRatio: false,
    responsive: true,
    interaction: {
      intersect: false,
      mode: 'index',
    },
    plugins: {
      legend: {
        display: true,
        labels: {
          boxHeight: 8,
          boxWidth: 8,
          color: '#687385',
          font: {
            size: 11,
            weight: 700,
          },
          usePointStyle: true,
        },
        position: 'bottom',
      },
      tooltip: {
        callbacks: {
          label(context) {
            return `${context.dataset.label}: ${formatNumber(Number(context.parsed.y || 0))}`
          },
          title(items) {
            const label = String(items[0]?.label || '')
            return formatBucketLabel(label, bucketSize)
          },
        },
      },
    },
    scales: {
      x: {
        grid: {
          color: '#eef1f5',
        },
        stacked: controls.stacked,
        ticks: {
          color: '#687385',
          maxRotation: 0,
          minRotation: 0,
          callback(value, index) {
            const label = this.getLabelForValue(typeof value === 'number' ? value : index)
            return formatBucketLabel(label, bucketSize)
          },
        },
      },
      y: {
        beginAtZero: true,
        grid: {
          color: '#eef1f5',
        },
        stacked: controls.stacked,
        ticks: {
          color: '#687385',
          callback(value) {
            return formatNumber(Number(value))
          },
        },
      },
    },
  }
}

function cumulativeValues(values: number[]) {
  let running = 0
  return values.map((value) => {
    running += value
    return running
  })
}

function seriesLabel(bucket: UsageBucket, groupBy: string[]) {
  if (groupBy.length === 0) {
    return 'Total'
  }

  const group = bucket.group || {}
  const values = groupBy
    .map((field) => [field, group[field] || (field === 'subject' ? bucket.subject : '')] as const)
    .filter(([, value]) => value !== '')

  if (values.length === 0) {
    return 'Ungrouped'
  }

  return values.map(([field, value]) => `${humanizeField(field)}: ${value}`).join(' / ')
}

function formatBucketLabel(value: string, bucketSize: string) {
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) {
    return value
  }

  if (bucketSize === 'hour') {
    return new Intl.DateTimeFormat(undefined, {
      day: 'numeric',
      hour: '2-digit',
      hourCycle: 'h23',
      minute: '2-digit',
      month: 'short',
      timeZone: 'UTC',
      timeZoneName: 'short',
      year: 'numeric',
    }).format(date)
  }

  if (bucketSize === 'month') {
    return new Intl.DateTimeFormat(undefined, {
      month: 'short',
      timeZone: 'UTC',
      year: 'numeric',
    }).format(date)
  }

  return new Intl.DateTimeFormat(undefined, {
    day: 'numeric',
    month: 'short',
    timeZone: 'UTC',
    year: 'numeric',
  }).format(date)
}

function transparentColor(hex: string, alpha: number) {
  const value = hex.replace('#', '')
  const red = parseInt(value.slice(0, 2), 16)
  const green = parseInt(value.slice(2, 4), 16)
  const blue = parseInt(value.slice(4, 6), 16)
  return `rgba(${red}, ${green}, ${blue}, ${alpha})`
}

function humanizeField(key: string) {
  return key
    .replace(/^metadata\./, '')
    .split(/[._-]/)
    .filter(Boolean)
    .map((part) => part.charAt(0).toUpperCase() + part.slice(1))
    .join(' ')
}
