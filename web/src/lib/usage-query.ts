import type { Field, Operator, RuleGroupType, RuleType } from 'react-querybuilder'

import type { Meter, UsageDimensionValue, UsageFilter, UsageFilterCondition } from '../api'
import { defaultQueryDates, localDateTimeToISO, normalizeInputDateTime } from './datetime'

export type MetadataTypes = Record<string, string>
export type MetadataLabels = Record<string, string>

let queryNodeID = 0

export function defaultFilterQuery(): RuleGroupType {
  const dates = defaultQueryDates()
  return {
    combinator: 'and',
    id: nextQueryNodeID('group'),
    rules: [
      queryRule('meter', '=', ''),
      queryRule('timestamp', '>=', dates.from),
      queryRule('timestamp', '<=', dates.to),
    ],
  }
}

export function buildFilterFields(
  metadataKeys: string[],
  meters: Meter[],
  dimensionValues: Record<string, UsageDimensionValue[]> = {},
  metadataTypes: MetadataTypes = {},
  metadataLabels: MetadataLabels = {},
): Field[] {
  return [
    { name: 'subject', label: 'Subject' },
    {
      name: 'meter',
      label: 'Meter',
      valueEditorType: 'select',
      values: meters.map((meter) => ({ name: meter.name, label: meter.name })),
    },
    { name: 'quantity', label: 'Quantity', inputType: 'number' },
    { name: 'timestamp', label: 'Timestamp', inputType: 'datetime-local' },
    { name: 'received_at', label: 'Received At', inputType: 'datetime-local' },
    { name: 'idempotency_key', label: 'Idempotency Key' },
    ...metadataKeys.map((key) => metadataFilterField(key, dimensionValues[key] || [], metadataTypes[`metadata.${key}`], metadataLabels[`metadata.${key}`])),
  ]
}

export function usageFilterFromQuery(query: RuleGroupType, metadataTypes: MetadataTypes = {}): UsageFilter | undefined {
  const rules = query.rules
    .map((rule) => isQueryGroup(rule) ? usageFilterFromQuery(rule, metadataTypes) : usageFilterFromRule(rule, metadataTypes))
    .filter((rule): rule is UsageFilter => rule !== undefined)

  if (rules.length === 0) {
    return undefined
  }
  if (rules.length === 1) {
    return rules[0]
  }
  return {
    type: 'group',
    op: query.combinator === 'or' ? 'or' : 'and',
    rules,
  }
}

export function usageScopeFromQuery(query: RuleGroupType) {
  const subject = firstEqualRuleValue(query, 'subject')
  const meter = firstEqualRuleValue(query, 'meter')
  if (!meter) {
    throw new Error('Usage query needs a meter filter')
  }
  return { meter, subject }
}

export function usageTimeRangeFromQuery(query: RuleGroupType) {
  const from = firstComparableRuleValue(query, 'timestamp', ['>=', '>'])
  const to = firstComparableRuleValue(query, 'timestamp', ['<=', '<'])
  if (!from || !to) {
    throw new Error('Usage query needs timestamp from and to filters')
  }
  return {
    from: localDateTimeToISO(from),
    to: localDateTimeToISO(to),
  }
}

export function queryWithAvailableMeter(query: RuleGroupType, meters: Meter[]): RuleGroupType {
  const availableMeters = new Set(meters.map((meter) => meter.name))
  const fallbackMeter = meters[0]?.name || ''
  if (!fallbackMeter) {
    return query
  }
  return replaceRuleValue(query, 'meter', (value) => availableMeters.has(value) ? value : fallbackMeter)
}

export function firstEqualRuleValue(query: RuleGroupType, field: string): string {
  for (const rule of query.rules) {
    if (isQueryGroup(rule)) {
      const value = firstEqualRuleValue(rule, field)
      if (value) {
        return value
      }
      continue
    }
    if (rule.field === field && rule.operator === '=' && rule.value) {
      return String(rule.value)
    }
  }
  return ''
}

export function selectedMeterSchemaKeys(meters: Meter[], selectedMeterName?: string) {
  const selectedMeter = meters.find((meter) => meter.name === selectedMeterName)
  if (selectedMeter) {
    return activeMeterDimensions(selectedMeter).map((dimension) => dimension.name).sort()
  }
  return Array.from(new Set(meters.flatMap((meter) => activeMeterDimensions(meter).map((dimension) => dimension.name)))).sort()
}

export function metadataTypesByField(meters: Meter[], selectedMeterName?: string): MetadataTypes {
  const selectedMeter = meters.find((meter) => meter.name === selectedMeterName)
  return Object.fromEntries(
    meterDimensions(selectedMeter).map((dimension) => [`metadata.${dimension.name}`, dimension.type]),
  )
}

export function metadataLabelsByField(meters: Meter[], selectedMeterName?: string): MetadataLabels {
  const selectedMeter = meters.find((meter) => meter.name === selectedMeterName)
  return Object.fromEntries(
    meterDimensions(selectedMeter).map((dimension) => [`metadata.${dimension.name}`, dimension.display_name || humanizeField(dimension.name)]),
  )
}

export function usageDimensionDiscoveryKey(query: RuleGroupType, meters: Meter[]) {
  const meter = firstEqualRuleValue(query, 'meter')
  const metadataKeys = selectedMeterSchemaKeys(meters, meter)
  if (!meter || metadataKeys.length === 0) {
    return ''
  }

  let from = ''
  let to = ''
  try {
    const range = usageTimeRangeFromQuery(query)
    from = range.from
    to = range.to
  } catch {
    // Discovery still works without a valid time range; the key just omits it.
  }

  return [
    meter,
    firstEqualRuleValue(query, 'subject'),
    from,
    to,
    metadataKeys.join(','),
  ].join('|')
}

export function usageBreakdownQueryKey(query: RuleGroupType, meters: Meter[]) {
  const meter = firstEqualRuleValue(query, 'meter')
  if (!meter) {
    return ''
  }

  let range: ReturnType<typeof usageTimeRangeFromQuery>
  try {
    range = usageTimeRangeFromQuery(query)
  } catch {
    return ''
  }

  return [
    meter,
    range.from,
    range.to,
    selectedMeterSchemaKeys(meters, meter).join(','),
    JSON.stringify(query.rules),
  ].join('|')
}

export function getFilterOperators(field: string, metadataTypes: MetadataTypes = {}): Operator[] {
  const metadataType = metadataTypes[field]
  if (field === 'quantity' || field === 'timestamp' || field === 'received_at' || metadataType === 'number') {
    return [
      { name: '=', label: 'equals' },
      { name: '!=', label: 'not equals' },
      { name: '>', label: 'greater than' },
      { name: '>=', label: 'greater or equal' },
      { name: '<', label: 'less than' },
      { name: '<=', label: 'less or equal' },
    ]
  }
  if (metadataType === 'boolean') {
    return [
      { name: '=', label: 'equals' },
      { name: '!=', label: 'not equals' },
      { name: 'notNull', label: 'exists', arity: 'unary' },
    ]
  }

  return [
    { name: '=', label: 'equals' },
    { name: '!=', label: 'not equals' },
    { name: 'contains', label: 'contains' },
    { name: 'in', label: 'in list' },
    { name: 'notNull', label: 'exists', arity: 'unary' },
  ]
}

export function getFilterInputType(field: string, metadataTypes: MetadataTypes = {}) {
  if (field === 'quantity' || metadataTypes[field] === 'number') {
    return 'number'
  }
  if (field === 'timestamp' || field === 'received_at') {
    return 'datetime-local'
  }
  return 'text'
}

export function countQueryRules(query: RuleGroupType): number {
  return query.rules.reduce((sum, rule) => sum + (isQueryGroup(rule) ? countQueryRules(rule) : 1), 0)
}

export function queryFromSavedValue(value: unknown, fallback: RuleGroupType): RuleGroupType {
  if (!value || typeof value !== 'object' || Array.isArray(value)) {
    return fallback
  }

  const candidate = value as Partial<RuleGroupType>
  if (!Array.isArray(candidate.rules)) {
    return fallback
  }

  return {
    id: typeof candidate.id === 'string' && candidate.id ? candidate.id : nextQueryNodeID('group'),
    combinator: candidate.combinator === 'or' ? 'or' : 'and',
    rules: candidate.rules.map(normalizeSavedQueryRule),
  }
}

export function queryWithBreakdownFilter(query: RuleGroupType, field: string, value: string): RuleGroupType {
  const filterField = field === 'subject' ? 'subject' : `metadata.${field.replace(/^metadata\./, '')}`

  return replaceRuleValue(query, filterField, () => value)
}

export function queryWithSubject(query: RuleGroupType, subject: string): RuleGroupType {
  return replaceRuleValue(query, 'subject', () => subject)
}

export function queryWithMeter(query: RuleGroupType, meter: string): RuleGroupType {
  return replaceRuleValue(query, 'meter', () => meter)
}

function metadataFilterField(key: string, values: UsageDimensionValue[], metadataType?: string, label?: string): Field {
  const options = values.map((item) => ({
    name: item.value,
    label: `${item.value} (${item.events})`,
  }))

  if (metadataType === 'boolean' && options.length === 0) {
    options.push(
      { name: 'true', label: 'true' },
      { name: 'false', label: 'false' },
    )
  }

  return {
    name: `metadata.${key}`,
    label: label || humanizeField(key),
    ...(options.length > 0 ? { valueEditorType: 'select' as const, values: options } : {}),
  }
}

function meterDimensions(meter?: Meter) {
  if (!meter) {
    return []
  }
  if (meter.dimensions && meter.dimensions.length > 0) {
    return meter.dimensions
  }
  return Object.entries(meter.metadata_schema || {}).map(([name, type]) => ({
    description: '',
    display_name: humanizeField(name),
    deprecated: false,
    name,
    required: true,
    type,
  }))
}

function activeMeterDimensions(meter?: Meter) {
  return meterDimensions(meter).filter((dimension) => !dimension.deprecated)
}

function humanizeField(key: string) {
  return key
    .replace(/^metadata\./, '')
    .split(/[._-]/)
    .filter(Boolean)
    .map((part) => part.charAt(0).toUpperCase() + part.slice(1))
    .join(' ')
}

function usageFilterFromRule(rule: RuleType, metadataTypes: MetadataTypes): UsageFilter | undefined {
  if (!rule.field || !rule.operator) {
    return undefined
  }

  const op = usageOperatorFromQueryOperator(rule.operator)
  if (!op) {
    return undefined
  }

  const value = usageValueFromRule(rule, metadataTypes)
  if (op !== 'exists' && value === undefined) {
    return undefined
  }

  return {
    type: 'condition',
    field: rule.field,
    op,
    value,
  }
}

function usageOperatorFromQueryOperator(operator: string): UsageFilterCondition['op'] | undefined {
  switch (operator) {
    case '=':
      return 'eq'
    case '!=':
      return 'neq'
    case '>':
      return 'gt'
    case '>=':
      return 'gte'
    case '<':
      return 'lt'
    case '<=':
      return 'lte'
    case 'in':
      return 'in'
    case 'contains':
      return 'contains'
    case 'notNull':
      return 'exists'
    default:
      return undefined
  }
}

function usageValueFromRule(rule: RuleType, metadataTypes: MetadataTypes) {
  if (rule.operator === 'notNull') {
    return undefined
  }
  const metadataType = metadataTypes[rule.field]
  if (rule.operator === 'in') {
    const values = Array.isArray(rule.value)
      ? rule.value
      : String(rule.value || '').split(',').map((value) => value.trim()).filter(Boolean)
    const typedValues = values
      .map((value) => typedMetadataValue(value, metadataType))
      .filter((value) => value !== undefined)
    return typedValues.length > 0 ? typedValues : undefined
  }
  if (rule.field === 'timestamp' || rule.field === 'received_at') {
    return localDateTimeToISO(String(rule.value || '')) || undefined
  }
  if (rule.field === 'quantity') {
    return rule.value === '' || rule.value === undefined ? undefined : Number(rule.value)
  }
  return typedMetadataValue(rule.value, metadataType)
}

function typedMetadataValue(value: unknown, metadataType?: string) {
  if (value === '' || value === undefined) {
    return undefined
  }
  if (metadataType === 'number') {
    const parsed = Number(value)
    return Number.isFinite(parsed) ? parsed : undefined
  }
  if (metadataType === 'boolean') {
    if (typeof value === 'boolean') {
      return value
    }
    const normalized = String(value).trim().toLowerCase()
    if (normalized === 'true') {
      return true
    }
    if (normalized === 'false') {
      return false
    }
    return undefined
  }
  return value
}

function normalizeSavedQueryRule(rule: RuleGroupType['rules'][number]): RuleGroupType['rules'][number] {
  if (isQueryGroup(rule)) {
    return {
      ...rule,
      combinator: rule.combinator === 'or' ? 'or' : 'and',
      id: rule.id || nextQueryNodeID('group'),
      rules: rule.rules.map(normalizeSavedQueryRule),
    }
  }
  const id = rule.id || nextQueryNodeID('rule')
  if ((rule.field === 'timestamp' || rule.field === 'received_at') && typeof rule.value === 'string') {
    return {
      ...rule,
      id,
      value: normalizeInputDateTime(rule.value),
    }
  }
  return { ...rule, id }
}

function replaceRuleValue(query: RuleGroupType, field: string, nextValue: (value: string) => string): RuleGroupType {
  let replaced = false
  const rules = query.rules.map((rule) => {
    if (isQueryGroup(rule)) {
      return replaceRuleValue(rule, field, nextValue)
    }
    if (!replaced && rule.field === field && rule.operator === '=') {
      replaced = true
      return { ...rule, value: nextValue(String(rule.value || '')) }
    }
    return rule
  })

  if (replaced) {
    return { ...query, rules }
  }
  return {
    ...query,
    rules: [...rules, queryRule(field, '=', nextValue(''))],
  }
}

function firstComparableRuleValue(query: RuleGroupType, field: string, operators: string[]): string {
  for (const rule of query.rules) {
    if (isQueryGroup(rule)) {
      const value = firstComparableRuleValue(rule, field, operators)
      if (value) {
        return value
      }
      continue
    }
    if (rule.field === field && operators.includes(rule.operator) && rule.value) {
      return String(rule.value)
    }
  }
  return ''
}

function isQueryGroup(rule: RuleGroupType['rules'][number]): rule is RuleGroupType {
  return Boolean(rule && typeof rule === 'object' && 'rules' in rule)
}

function queryRule(field: string, operator: string, value: unknown): RuleType {
  return {
    field,
    id: nextQueryNodeID('rule'),
    operator,
    value,
  }
}

function nextQueryNodeID(prefix: string) {
  queryNodeID += 1
  return `${prefix}-${queryNodeID}`
}
