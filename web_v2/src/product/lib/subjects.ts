const subjectIdentifierPattern = /^[A-Za-z0-9][A-Za-z0-9_.:-]{0,127}$/

export function normalizeSubjectIdentifier(value: string) {
  return value.trim()
}

export function isValidSubjectIdentifier(value: string) {
  return subjectIdentifierPattern.test(normalizeSubjectIdentifier(value))
}
