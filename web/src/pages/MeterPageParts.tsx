import type { Meter, MeterDimension } from '../api'
import { Badge } from '../components/ui/badge'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '../components/ui/table'

export function DimensionChips({ meter }: { meter: Meter }) {
  const dimensions = normalizedMeterDimensions(meter)
  if (dimensions.length === 0) {
    return <span className="muted">No dimensions</span>
  }

  return (
    <div className="schema-chips">
      {dimensions.map((dimension) => (
        <span className="schema-chip" key={dimension.name}>
          <span>{dimension.display_name || humanizeField(dimension.name)}</span>
          <strong>{dimension.deprecated ? `${dimension.type} deprecated` : dimension.required ? dimension.type : `${dimension.type} optional`}</strong>
        </span>
      ))}
    </div>
  )
}

export function DimensionTable({ meter }: { meter: Meter }) {
  const dimensions = normalizedMeterDimensions(meter)

  if (dimensions.length === 0) {
    return <p className="subject-empty">This meter does not define dimensions.</p>
  }

  return (
    <div className="table-wrap">
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead>Name</TableHead>
            <TableHead>Display</TableHead>
            <TableHead>Type</TableHead>
            <TableHead>Required</TableHead>
            <TableHead>Status</TableHead>
            <TableHead>Description</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {dimensions.map((dimension) => (
            <TableRow key={dimension.name}>
              <TableCell><span className="mono">{dimension.name}</span></TableCell>
              <TableCell>{dimension.display_name || humanizeField(dimension.name)}</TableCell>
              <TableCell><Badge variant="muted">{dimension.type}</Badge></TableCell>
              <TableCell>{dimension.required ? <Badge variant="warning">Required</Badge> : <Badge variant="muted">Optional</Badge>}</TableCell>
              <TableCell>{dimension.deprecated ? <Badge variant="warning">Deprecated</Badge> : <Badge variant="success">Active</Badge>}</TableCell>
              <TableCell>{dimension.description || <span className="muted">No description</span>}</TableCell>
            </TableRow>
          ))}
        </TableBody>
      </Table>
    </div>
  )
}

function normalizedMeterDimensions(meter: Meter): MeterDimension[] {
  return meter.dimensions || []
}

function humanizeField(key: string) {
  return key
    .replace(/^metadata\./, '')
    .split(/[._-]/)
    .filter(Boolean)
    .map((part) => part.charAt(0).toUpperCase() + part.slice(1))
    .join(' ')
}
