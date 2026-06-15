export function parseMetadataSchema(value: string) {
  const parsed: unknown = JSON.parse(value || '{}')
  if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) {
    throw new Error('Metadata schema must be a JSON object')
  }
  return Object.fromEntries(Object.entries(parsed).map(([key, schemaValue]) => [key, String(schemaValue)]))
}

const metadataNamePattern = /^[A-Za-z0-9_]+(\.[A-Za-z0-9_]+)*$/
const metadataTypes = new Set(['string', 'number', 'boolean'])

export function metadataSchemaFromRows(rows: Array<{ name: string; type: string }>) {
  return metadataSchemaFromDimensions(meterDimensionsFromRows(rows))
}

export function metadataSchemaFromDimensions(dimensions: Array<{ name: string; type: string }>) {
  const schema: Record<string, string> = {}

  for (const dimension of dimensions) {
    schema[dimension.name] = dimension.type
  }

  return schema
}

export function meterDimensionsFromRows(rows: Array<{
  description?: string
  displayName?: string
  name: string
  required?: boolean
  type: string
}>) {
  const names = new Set<string>()
  const dimensions = []

  for (const row of rows) {
    const name = row.name.trim()
    if (!name) {
      continue
    }
    if (!metadataNamePattern.test(name)) {
      throw new Error('Dimension names can use letters, numbers, underscores, and dots')
    }
    if (names.has(name)) {
      throw new Error(`Dimension "${name}" is already defined`)
    }
    if (!metadataTypes.has(row.type)) {
      throw new Error(`Dimension "${name}" has an unsupported type`)
    }
    names.add(name)
    dimensions.push({
      description: row.description?.trim() || '',
      display_name: row.displayName?.trim() || '',
      name,
      required: row.required ?? true,
      type: row.type,
    })
  }

  return dimensions
}

export function parseJSONRecord(value: string, label: string) {
  const parsed: unknown = JSON.parse(value || '{}')
  if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) {
    throw new Error(`${label} must be a JSON object`)
  }
  return parsed as Record<string, unknown>
}
