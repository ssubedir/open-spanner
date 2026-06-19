package auth

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/casbin/casbin/v2"
	"github.com/casbin/casbin/v2/model"

	"github.com/ssubedir/open-spanner/internal/metering/domain"
)

type PrincipalKind string

const (
	PrincipalKindSession PrincipalKind = "session"
	PrincipalKindAPIKey  PrincipalKind = "api_key"
)

type Action string

const (
	ActionUsageRead    Action = "usage:read"
	ActionUsageWrite   Action = "usage:write"
	ActionMetersRead   Action = "meters:read"
	ActionMetersWrite  Action = "meters:write"
	ActionAlertsRead   Action = "alerts:read"
	ActionAlertsWrite  Action = "alerts:write"
	ActionExportsRead  Action = "exports:read"
	ActionExportsWrite Action = "exports:write"
	ActionSystemRead   Action = "system:read"
)

const (
	ResourceUsage  = "usage"
	ResourceMeter  = "meter"
	ResourceAlert  = "alert"
	ResourceExport = "export"
	ResourceSystem = "system"
)

var DefaultAPIKeyScopes = []string{
	string(ActionUsageWrite),
	string(ActionUsageRead),
	string(ActionMetersRead),
	string(ActionMetersWrite),
}

var allowedAPIKeyScopes = map[string]struct{}{
	string(ActionUsageWrite):   {},
	string(ActionUsageRead):    {},
	string(ActionMetersRead):   {},
	string(ActionMetersWrite):  {},
	string(ActionAlertsRead):   {},
	string(ActionAlertsWrite):  {},
	string(ActionExportsRead):  {},
	string(ActionExportsWrite): {},
	string(ActionSystemRead):   {},
	"usage:*":                  {},
	"meters:*":                 {},
	"alerts:*":                 {},
	"exports:*":                {},
	"system:*":                 {},
	"*":                        {},
}

type Principal struct {
	Kind          PrincipalKind
	ID            string
	User          UserResult
	APIKeyID      string
	Scopes        []string
	AllowedMeters []string
	ExpiresAt     *time.Time
	RevokedAt     *time.Time
}

type Resource struct {
	Type       string
	ID         string
	Meter      string
	Subject    string
	Attributes map[string]string
}

type Authorizer interface {
	Can(ctx context.Context, principal Principal, action Action, resource Resource) error
}

type CasbinAuthorizer struct {
	enforcer *casbin.SyncedEnforcer
}

func NewCasbinAuthorizer() (*CasbinAuthorizer, error) {
	m, err := model.NewModelFromString(`
[request_definition]
r = sub, obj, act

[policy_definition]
p = sub, obj, act

[policy_effect]
e = some(where (p.eft == allow))

[matchers]
m = (p.sub == "*" || p.sub == r.sub) && (p.obj == "*" || p.obj == r.obj) && (p.act == "*" || p.act == r.act)
`)
	if err != nil {
		return nil, err
	}

	enforcer, err := casbin.NewSyncedEnforcer(m)
	if err != nil {
		return nil, err
	}

	for _, policy := range [][]string{
		{string(PrincipalKindSession), "*", "*"},
		{string(PrincipalKindAPIKey), ResourceUsage, string(ActionUsageRead)},
		{string(PrincipalKindAPIKey), ResourceUsage, string(ActionUsageWrite)},
		{string(PrincipalKindAPIKey), ResourceMeter, string(ActionMetersRead)},
		{string(PrincipalKindAPIKey), ResourceMeter, string(ActionMetersWrite)},
		{string(PrincipalKindAPIKey), ResourceAlert, string(ActionAlertsRead)},
		{string(PrincipalKindAPIKey), ResourceAlert, string(ActionAlertsWrite)},
		{string(PrincipalKindAPIKey), ResourceExport, string(ActionExportsRead)},
		{string(PrincipalKindAPIKey), ResourceExport, string(ActionExportsWrite)},
		{string(PrincipalKindAPIKey), ResourceSystem, string(ActionSystemRead)},
	} {
		if _, err := enforcer.AddPolicy(policy); err != nil {
			return nil, err
		}
	}

	return &CasbinAuthorizer{enforcer: enforcer}, nil
}

func (a *CasbinAuthorizer) Can(_ context.Context, principal Principal, action Action, resource Resource) error {
	if a == nil || a.enforcer == nil {
		return nil
	}

	allowed, err := a.enforcer.Enforce(string(principal.Kind), resource.Type, string(action))
	if err != nil {
		return err
	}
	if !allowed {
		return forbidden(action, resource)
	}

	if principal.Kind == PrincipalKindSession {
		return nil
	}
	if principal.RevokedAt != nil {
		return errors.Join(domain.ErrForbidden, errors.New("api key is revoked"))
	}
	if principal.ExpiresAt != nil && !principal.ExpiresAt.After(time.Now().UTC()) {
		return errors.Join(domain.ErrForbidden, errors.New("api key is expired"))
	}
	if !scopeAllows(principal.Scopes, string(action)) {
		return forbidden(action, resource)
	}
	if !meterAllows(principal.AllowedMeters, resource.Meter) {
		return errors.Join(domain.ErrForbidden, fmt.Errorf("api key cannot access meter %q", resource.Meter))
	}
	return nil
}

func normalizeAPIKeyScopes(input []string) ([]string, error) {
	values := input
	if len(values) == 0 {
		values = DefaultAPIKeyScopes
	}

	seen := map[string]struct{}{}
	scopes := make([]string, 0, len(values))
	for _, value := range values {
		scope := strings.ToLower(strings.TrimSpace(value))
		if scope == "" {
			continue
		}
		if _, ok := allowedAPIKeyScopes[scope]; !ok {
			return nil, errors.Join(domain.ErrInvalidInput, fmt.Errorf("unsupported api key scope %q", value))
		}
		if _, ok := seen[scope]; ok {
			continue
		}
		seen[scope] = struct{}{}
		scopes = append(scopes, scope)
	}
	if len(scopes) == 0 {
		return nil, errors.Join(domain.ErrInvalidInput, errors.New("at least one api key scope is required"))
	}
	return scopes, nil
}

func normalizeAllowedMeters(input []string) []string {
	seen := map[string]struct{}{}
	meters := make([]string, 0, len(input))
	for _, value := range input {
		meter := strings.TrimSpace(value)
		if meter == "" {
			continue
		}
		if _, ok := seen[meter]; ok {
			continue
		}
		seen[meter] = struct{}{}
		meters = append(meters, meter)
	}
	return meters
}

func scopeAllows(scopes []string, action string) bool {
	for _, scope := range scopes {
		switch scope {
		case "*", action:
			return true
		}
		if prefix, _, ok := strings.Cut(action, ":"); ok && scope == prefix+":*" {
			return true
		}
	}
	return false
}

func meterAllows(allowedMeters []string, meter string) bool {
	if len(allowedMeters) == 0 {
		return true
	}
	if strings.TrimSpace(meter) == "" {
		return false
	}
	for _, allowedMeter := range allowedMeters {
		if allowedMeter == "*" || allowedMeter == meter {
			return true
		}
	}
	return false
}

func forbidden(action Action, resource Resource) error {
	return errors.Join(domain.ErrForbidden, fmt.Errorf("not allowed to %s %s", action, resource.Type))
}
