import { useEffect, useState } from 'react'
import { QueryBuilder, ValueEditor, type Field, type RuleGroupType, type ValueEditorProps } from 'react-querybuilder'
import 'react-querybuilder/dist/query-builder.css'

import { cn } from '@/lib/utils'

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
        controlElements={{ valueEditor: FilterValueEditor }}
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

function FilterValueEditor(props: ValueEditorProps) {
  if (props.inputType !== 'datetime-local') {
    return <ValueEditor {...props} />
  }
  return <DateTimeValueEditor {...props} />
}

function DateTimeValueEditor({ className, disabled, handleOnChange, title, value }: ValueEditorProps) {
  const normalizedValue = normalizeInputDateTime(value)
  const [draft, setDraft] = useState(normalizedValue)

  useEffect(() => {
    setDraft(normalizedValue)
  }, [normalizedValue])

  return (
    <input
      className={className}
      disabled={disabled}
      onBlur={() => {
        const normalizedDraft = normalizeInputDateTime(draft)
        if (normalizedDraft && isCompleteInputDateTime(normalizedDraft)) {
          setDraft(normalizedDraft)
          handleOnChange(normalizedDraft)
          return
        }
        setDraft(normalizedValue)
      }}
      onChange={(event) => {
        const next = event.target.value
        setDraft(next)
        if (next === '' || isCompleteInputDateTime(next)) {
          handleOnChange(next)
        }
      }}
      title={title}
      type="datetime-local"
      value={draft}
    />
  )
}
