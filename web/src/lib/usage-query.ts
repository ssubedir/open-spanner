import type { Field, Operator, RuleGroupType, RuleType } from 'react-querybuilder'

import type { Meter, UsageFilter, UsageFilterCondition } from '../api'
import { defaultQueryDates, localDateTimeToISO } from './datetime'

export function defaultFilterQuery(): RuleGroupType {
  const dates = defaultQueryDates()
  return {
    combinator: 'and',
    rules: [
      { field: 'subject', operator: '=', value: 'org_123' },
      { field: 'meter', operator: '=', value: '' },
      { field: 'timestamp', operator: '>=', value: dates.from },
      { field: 'timestamp', operator: '<=', value: dates.to },
    ],
  }
}

export function buildFilterFields(metadataKeys: string[], meters: Meter[]): Field[] {
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
    ...metadataKeys.map((key) => ({ name: `metadata.${key}`, label: `Metadata: ${key}` })),
  ]
}

export function usageFilterFromQuery(query: RuleGroupType): UsageFilter | undefined {
  const rules = query.rules
    .map((rule) => isQueryGroup(rule) ? usageFilterFromQuery(rule) : usageFilterFromRule(rule))
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
  if (!subject || !meter) {
    throw new Error('Usage query needs subject and meter filters')
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
    return Object.keys(selectedMeter.metadata_schema || {}).sort()
  }
  return Array.from(new Set(meters.flatMap((meter) => Object.keys(meter.metadata_schema || {})))).sort()
}

export function getFilterOperators(field: string): Operator[] {
  if (field === 'quantity' || field === 'timestamp' || field === 'received_at') {
    return [
      { name: '=', label: 'equals' },
      { name: '!=', label: 'not equals' },
      { name: '>', label: 'greater than' },
      { name: '>=', label: 'greater or equal' },
      { name: '<', label: 'less than' },
      { name: '<=', label: 'less or equal' },
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

export function getFilterInputType(field: string) {
  if (field === 'quantity') {
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

function usageFilterFromRule(rule: RuleType): UsageFilter | undefined {
  if (!rule.field || !rule.operator) {
    return undefined
  }

  const op = usageOperatorFromQueryOperator(rule.operator)
  if (!op) {
    return undefined
  }

  const value = usageValueFromRule(rule)
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

function usageValueFromRule(rule: RuleType) {
  if (rule.operator === 'notNull') {
    return undefined
  }
  if (rule.operator === 'in') {
    return Array.isArray(rule.value)
      ? rule.value
      : String(rule.value || '').split(',').map((value) => value.trim()).filter(Boolean)
  }
  if (rule.field === 'timestamp' || rule.field === 'received_at') {
    return localDateTimeToISO(String(rule.value || '')) || undefined
  }
  if (rule.field === 'quantity') {
    return rule.value === '' || rule.value === undefined ? undefined : Number(rule.value)
  }
  return rule.value === '' || rule.value === undefined ? undefined : rule.value
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
    rules: [...rules, { field, operator: '=', value: nextValue('') }],
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
