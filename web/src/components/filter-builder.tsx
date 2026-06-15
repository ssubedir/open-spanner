import { QueryBuilder, type Field, type RuleGroupType } from 'react-querybuilder'
import 'react-querybuilder/dist/query-builder.css'

import { countQueryRules, getFilterInputType, getFilterOperators, type MetadataTypes } from '../lib/usage-query'

export function FilterBuilder({
  fields,
  metadataTypes = {},
  onChange,
  query,
}: {
  fields: Field[]
  metadataTypes?: MetadataTypes
  onChange: (query: RuleGroupType) => void
  query: RuleGroupType
}) {
  return (
    <div className="filter-builder wide">
      <div className="filter-builder-header">
        <div>
          <span>Advanced Filters</span>
          <small>{countQueryRules(query)} active</small>
        </div>
      </div>
      <QueryBuilder
        fields={fields}
        getInputType={(field) => getFilterInputType(field, metadataTypes)}
        getOperators={(field) => getFilterOperators(field, metadataTypes)}
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
