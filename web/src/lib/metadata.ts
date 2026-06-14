export function parseMetadataSchema(value: string) {
  const parsed: unknown = JSON.parse(value || '{}')
  if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) {
    throw new Error('Metadata schema must be a JSON object')
  }
  return Object.fromEntries(Object.entries(parsed).map(([key, schemaValue]) => [key, String(schemaValue)]))
}

export function parseJSONRecord(value: string, label: string) {
  const parsed: unknown = JSON.parse(value || '{}')
  if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) {
    throw new Error(`${label} must be a JSON object`)
  }
  return parsed as Record<string, unknown>
}
