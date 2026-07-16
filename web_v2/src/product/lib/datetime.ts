const inputDateTimePattern = /^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}$/
const localDateTimeWithSecondsPattern = /^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}(?::\d{2}(?:\.\d+)?)?$/

export function isCompleteInputDateTime(value: string) {
  return inputDateTimePattern.test(value)
}

export function normalizeInputDateTime(value: unknown) {
  const raw = String(value ?? '').trim()
  if (!raw) {
    return ''
  }
  if (inputDateTimePattern.test(raw)) {
    return raw
  }
  if (localDateTimeWithSecondsPattern.test(raw)) {
    return raw.slice(0, 16)
  }

  const parsed = new Date(raw)
  if (Number.isNaN(parsed.getTime())) {
    return ''
  }
  return toInputDateTime(parsed)
}

export function localDateTimeToISO(value: string) {
  const normalized = normalizeInputDateTime(value)
  if (!isCompleteInputDateTime(normalized)) {
    return ''
  }

  const parsed = new Date(normalized)
  if (Number.isNaN(parsed.getTime())) {
    return ''
  }
  return parsed.toISOString()
}

export function toInputDateTime(date: Date) {
  const local = new Date(date.getTime() - date.getTimezoneOffset() * 60000)
  return local.toISOString().slice(0, 16)
}

export function defaultQueryDates() {
  const now = new Date()
  const from = new Date(now)
  from.setDate(now.getDate() - 7)
  return {
    from: toInputDateTime(from),
    to: toInputDateTime(now),
  }
}
