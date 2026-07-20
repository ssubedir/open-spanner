const metadataNamePattern = /^[A-Za-z0-9_-]+(\.[A-Za-z0-9_-]+)*$/
const reservedMetadataNames = new Set(['subject'])
const metadataTypes = new Set(['string', 'number', 'boolean'])

export function metadataDimensionNameError(name: string) {
  const trimmedName = name.trim()
  if (!trimmedName) {
    return ''
  }
  if (!metadataNamePattern.test(trimmedName)) {
    return 'Use letters, numbers, underscores, hyphens, or dots.'
  }
  if (reservedMetadataNames.has(trimmedName)) {
    return '"subject" is reserved for the built-in subject field.'
  }
  return ''
}

export function meterDimensionsFromRows(rows: Array<{
  deprecated?: boolean
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
    const nameError = metadataDimensionNameError(name)
    if (nameError) {
      throw new Error(`Dimension "${name}" is invalid. ${nameError}`)
    }
    if (names.has(name)) {
      throw new Error(`Dimension "${name}" is already defined`)
    }
    if (!metadataTypes.has(row.type)) {
      throw new Error(`Dimension "${name}" has an unsupported type`)
    }
    names.add(name)
    dimensions.push({
      deprecated: row.deprecated ?? false,
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
