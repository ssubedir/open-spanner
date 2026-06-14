export function localDateTimeToISO(value: string) {
  if (!value) {
    return ''
  }
  return new Date(value).toISOString()
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
