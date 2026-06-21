import { useEffect } from 'react'
import {
  QueryBuilder,
  type ActionProps,
  type Field,
  type RuleGroupType,
  type ValueEditorProps,
  type ValueSelectorProps,
} from 'react-querybuilder'
import 'react-querybuilder/dist/query-builder.css'

import { cn } from '@/lib/utils'

import { Button } from './ui/button'
import { Input } from './ui/input'
import { Select, SelectContent, SelectGroup, SelectItem, SelectLabel, SelectTrigger, SelectValue } from './ui/select'
import { isCompleteInputDateTime, normalizeInputDateTime } from '../lib/datetime'
import { countQueryRules, getFilterInputType, getFilterOperators, type MetadataTypes } from '../lib/usage-query'

export function FilterBuilder({
  className,
  fields,
  metadataTypes = {},
  onChange,
  query,
}: {
  className?: string
  fields: Field[]
  metadataTypes?: MetadataTypes
  onChange: (query: RuleGroupType) => void
  query: RuleGroupType
}) {
  return (
    <div className={cn('filter-builder', className)}>
      <div className="filter-builder-header">
        <div>
          <span>Filters</span>
          <small>{countQueryRules(query)} active</small>
        </div>
      </div>
      <QueryBuilder
        fields={fields}
        getInputType={(field) => getFilterInputType(field, metadataTypes)}
        getOperators={(field) => getFilterOperators(field, metadataTypes)}
        controlElements={{
          actionElement: FilterAction,
          addGroupAction: FilterAction,
          addRuleAction: FilterAction,
          combinatorSelector: FilterValueSelector,
          fieldSelector: FilterValueSelector,
          operatorSelector: FilterValueSelector,
          removeRuleAction: FilterAction,
          valueEditor: FilterValueEditor,
          valueSelector: FilterValueSelector,
          valueSourceSelector: FilterValueSelector,
        }}
        listsAsArrays
        onQueryChange={onChange}
        parseNumbers="native"
        query={query}
        translations={{
          addGroup: { label: '+ Group' },
          addRule: { label: '+ Rule' },
        }}
      />
    </div>
  )
}

function FilterAction({ className, disabled, handleOnClick, label, title }: ActionProps) {
  return (
    <Button
      className={className}
      disabled={disabled}
      onClick={(event) => handleOnClick(event)}
      size="sm"
      title={title}
      type="button"
      variant="outline"
    >
      {label}
    </Button>
  )
}

function FilterValueSelector({ className, disabled, handleOnChange, options, title, value }: ValueSelectorProps) {
  const selectedValue = value == null ? '' : String(value)
  const fallbackValue = firstSelectableOption(options)

  useEffect(() => {
    if (!disabled && !selectedValue && fallbackValue) {
      handleOnChange(fallbackValue)
    }
  }, [disabled, fallbackValue, handleOnChange, selectedValue])

  return (
    <Select
      disabled={disabled}
      onValueChange={handleOnChange}
      value={selectedValue || undefined}
    >
      <SelectTrigger className={cn('filter-builder-select w-full justify-between', className)} title={title}>
        <SelectValue placeholder="Select" />
      </SelectTrigger>
      <SelectContent align="start" position="popper">
        {options.map((option) => isOptionGroup(option) ? (
          <SelectGroup key={option.label}>
            <SelectLabel>{option.label}</SelectLabel>
            {option.options.map((groupOption) => <FilterSelectItem key={optionValue(groupOption)} option={groupOption} />)}
          </SelectGroup>
        ) : (
          <FilterSelectItem key={optionValue(option)} option={option} />
        ))}
      </SelectContent>
    </Select>
  )
}

function FilterSelectItem({ option }: { option: FilterOption }) {
  const value = optionValue(option)
  if (!value) {
    return null
  }
  return (
    <SelectItem disabled={option.disabled} value={value}>
      {optionLabel(option)}
    </SelectItem>
  )
}

function FilterValueEditor(props: ValueEditorProps) {
  if (props.operator === 'null' || props.operator === 'notNull') {
    return null
  }
  if (props.type === 'select' || (props.values && props.values.length > 0)) {
    return <FilterValueSelector {...props} options={props.values ?? []} />
  }
  if (props.inputType === 'datetime-local') {
    return <DateTimeValueEditor {...props} />
  }
  return <InputValueEditor {...props} />
}

function InputValueEditor({ className, disabled, handleOnChange, inputType, title, value }: ValueEditorProps) {
  return (
    <Input
      className={className}
      disabled={disabled}
      onChange={(event) => {
        const next = event.currentTarget.value
        handleOnChange(inputType === 'number' && next !== '' ? Number(next) : next)
      }}
      title={title}
      type={inputType || 'text'}
      value={value ?? ''}
    />
  )
}

function DateTimeValueEditor({ className, disabled, handleOnChange, title, value }: ValueEditorProps) {
  const normalizedValue = normalizeInputDateTime(value)

  return (
    <Input
      className={className}
      disabled={disabled}
      onBlur={(event) => {
        const next = event.currentTarget.value
        const normalizedDraft = normalizeInputDateTime(next)
        if (normalizedDraft && isCompleteInputDateTime(normalizedDraft)) {
          handleOnChange(normalizedDraft)
          return
        }
        handleOnChange(normalizedValue)
      }}
      onChange={(event) => {
        handleOnChange(event.currentTarget.value)
      }}
      title={title}
      type="datetime-local"
      value={normalizedValue}
    />
  )
}

type FilterOption = {
  disabled?: boolean
  label?: string
  name?: string
  options?: FilterOption[]
  value?: string
}

function isOptionGroup(option: FilterOption): option is FilterOption & { label: string; options: FilterOption[] } {
  return Array.isArray(option.options)
}

function optionValue(option: FilterOption) {
  return String(option.value ?? option.name ?? '')
}

function optionLabel(option: FilterOption) {
  return String(option.label ?? option.name ?? option.value ?? '')
}

function firstSelectableOption(options: FilterOption[]): string {
  for (const option of options) {
    if (isOptionGroup(option)) {
      const groupValue = firstSelectableOption(option.options)
      if (groupValue) {
        return groupValue
      }
      continue
    }
    if (!option.disabled) {
      const value = optionValue(option)
      if (value) {
        return value
      }
    }
  }
  return ''
}
