package usage

import (
	"fmt"

	"github.com/ssubedir/open-spanner/internal/metering/domain"
	domainusage "github.com/ssubedir/open-spanner/internal/metering/domain/usage"
)

func filterFromRequest(req *FilterRequest) (domainusage.Filter, error) {
	if req == nil {
		return domainusage.EmptyFilter(), nil
	}

	switch domainusage.FilterType(req.Type) {
	case domainusage.FilterTypeGroup:
		rules := make([]domainusage.Filter, 0, len(req.Rules))
		for _, child := range req.Rules {
			rule, err := filterFromRequest(&child)
			if err != nil {
				return domainusage.Filter{}, err
			}
			rules = append(rules, rule)
		}
		return domainusage.NewFilterGroup(domainusage.FilterGroupOp(req.Op), rules)
	case domainusage.FilterTypeCondition:
		return domainusage.NewFilterCondition(req.Field, domainusage.FilterConditionOp(req.Op), req.Value, req.Value != nil)
	default:
		return domainusage.Filter{}, fmt.Errorf("%w: unsupported filter type %q", domain.ErrInvalidInput, req.Type)
	}
}
