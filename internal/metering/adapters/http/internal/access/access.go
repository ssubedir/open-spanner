package access

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"

	"github.com/go-chi/chi/v5"

	appauth "github.com/ssubedir/open-spanner/internal/auth"
	httpauth "github.com/ssubedir/open-spanner/internal/metering/adapters/http/auth"
	"github.com/ssubedir/open-spanner/internal/metering/adapters/http/internal/request"
	"github.com/ssubedir/open-spanner/internal/metering/adapters/http/internal/respond"
	"github.com/ssubedir/open-spanner/internal/metering/domain"
)

type Resource = appauth.Resource
type Authorizer = appauth.Authorizer

type ResourceExtractor func(*http.Request) ([]Resource, error)

type Policy struct {
	action    appauth.Action
	extractor ResourceExtractor
}

type Router struct {
	router     chi.Router
	authorizer Authorizer
}

type validationError struct {
	err error
}

func (e validationError) Error() string {
	return e.err.Error()
}

func (e validationError) Unwrap() error {
	return e.err
}

func middleware(authorizer Authorizer, action appauth.Action, extractor ResourceExtractor) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if authorizer == nil {
				next.ServeHTTP(w, r)
				return
			}

			resources, err := extractor(r)
			if err != nil {
				var validation validationError
				if errors.As(err, &validation) {
					respond.ValidationError(w, validation.err)
					return
				}
				var requestValidation request.ValidationError
				if errors.As(err, &requestValidation) {
					respond.ValidationError(w, err)
					return
				}
				respond.ServiceError(w, err)
				return
			}
			if len(resources) == 0 {
				resources = []Resource{{}}
			}

			for _, resource := range resources {
				if err := requireContext(r.Context(), authorizer, action, resource); err != nil {
					respond.ServiceError(w, err)
					return
				}
			}

			next.ServeHTTP(w, r)
		})
	}
}

func NewRouter(router chi.Router, authorizer Authorizer) Router {
	return Router{router: router, authorizer: authorizer}
}

func need(action appauth.Action, extractor ResourceExtractor) Policy {
	return Policy{action: action, extractor: extractor}
}

func UsageRead(extractor ResourceExtractor) Policy {
	return need(appauth.ActionUsageRead, extractor)
}

func UsageWrite(extractor ResourceExtractor) Policy {
	return need(appauth.ActionUsageWrite, extractor)
}

func MetersRead(extractor ResourceExtractor) Policy {
	return need(appauth.ActionMetersRead, extractor)
}

func MetersWrite(extractor ResourceExtractor) Policy {
	return need(appauth.ActionMetersWrite, extractor)
}

func AlertsRead(extractor ResourceExtractor) Policy {
	return need(appauth.ActionAlertsRead, extractor)
}

func AlertsWrite(extractor ResourceExtractor) Policy {
	return need(appauth.ActionAlertsWrite, extractor)
}

func ExportsRead(extractor ResourceExtractor) Policy {
	return need(appauth.ActionExportsRead, extractor)
}

func ExportsWrite(extractor ResourceExtractor) Policy {
	return need(appauth.ActionExportsWrite, extractor)
}

func PlansRead(extractor ResourceExtractor) Policy {
	return need(appauth.ActionPlansRead, extractor)
}

func PlansWrite(extractor ResourceExtractor) Policy {
	return need(appauth.ActionPlansWrite, extractor)
}

func SystemRead(extractor ResourceExtractor) Policy {
	return need(appauth.ActionSystemRead, extractor)
}

func (r Router) Route(pattern string, fn func(Router)) {
	r.router.Route(pattern, func(router chi.Router) {
		fn(NewRouter(router, r.authorizer))
	})
}

func (r Router) Get(pattern string, handler http.HandlerFunc, policies ...Policy) {
	r.router.With(r.authorization(policies...)...).Get(pattern, handler)
}

func (r Router) Post(pattern string, handler http.HandlerFunc, policies ...Policy) {
	r.router.With(r.authorization(policies...)...).Post(pattern, handler)
}

func (r Router) Put(pattern string, handler http.HandlerFunc, policies ...Policy) {
	r.router.With(r.authorization(policies...)...).Put(pattern, handler)
}

func (r Router) Delete(pattern string, handler http.HandlerFunc, policies ...Policy) {
	r.router.With(r.authorization(policies...)...).Delete(pattern, handler)
}

func (r Router) authorization(policies ...Policy) []func(http.Handler) http.Handler {
	middlewares := make([]func(http.Handler) http.Handler, 0, len(policies))
	for _, policy := range policies {
		middlewares = append(middlewares, middleware(r.authorizer, policy.action, policy.extractor))
	}
	return middlewares
}

func requireContext(ctx context.Context, authorizer Authorizer, action appauth.Action, resource Resource) error {
	if authorizer == nil {
		return nil
	}
	principal, ok := httpauth.PrincipalFromContext(ctx)
	if !ok {
		return domain.ErrUnauthorized
	}
	return authorizer.Can(ctx, principal, action, resource)
}

func Static(resource Resource) ResourceExtractor {
	return func(*http.Request) ([]Resource, error) {
		return Resources(resource), nil
	}
}

func Resources(resources ...Resource) []Resource {
	return resources
}

func Usage(meter string, subject string) Resource {
	return Resource{Type: appauth.ResourceUsage, Meter: meter, Subject: subject}
}

func Meter(meter string) Resource {
	return Resource{Type: appauth.ResourceMeter, Meter: meter}
}

func MeterByID(id string, meter string) Resource {
	return Resource{Type: appauth.ResourceMeter, ID: id, Meter: meter}
}

func Alert(meter string) Resource {
	return Resource{Type: appauth.ResourceAlert, Meter: meter}
}

func AlertByID(id string, meter string) Resource {
	return Resource{Type: appauth.ResourceAlert, ID: id, Meter: meter}
}

func Export(meter string) Resource {
	return Resource{Type: appauth.ResourceExport, Meter: meter}
}

func ExportByID(id string, meter string) Resource {
	return Resource{Type: appauth.ResourceExport, ID: id, Meter: meter}
}

func Plan(meter string) Resource {
	return Resource{Type: appauth.ResourcePlan, Meter: meter}
}

func PlanByID(id string, meter string) Resource {
	return Resource{Type: appauth.ResourcePlan, ID: id, Meter: meter}
}

func System() Resource {
	return Resource{Type: appauth.ResourceSystem}
}

func JSONBody[T any](mapper func(T) ([]Resource, error)) ResourceExtractor {
	return JSONBodyRequest(func(_ *http.Request, input T) ([]Resource, error) {
		return mapper(input)
	})
}

func JSONBodyRequest[T any](mapper func(*http.Request, T) ([]Resource, error)) ResourceExtractor {
	return func(r *http.Request) ([]Resource, error) {
		data, err := io.ReadAll(r.Body)
		if err != nil {
			return nil, err
		}
		_ = r.Body.Close()
		r.Body = io.NopCloser(bytes.NewReader(data))

		var input T
		if err := request.DecodeJSON(bytes.NewReader(data), &input); err != nil {
			return nil, validationError{err: err}
		}
		return mapper(r, input)
	}
}

func JSONBodyResource[T any](mapper func(T) (Resource, error)) ResourceExtractor {
	return JSONBody(func(input T) ([]Resource, error) {
		resource, err := mapper(input)
		if err != nil {
			return nil, err
		}
		return Resources(resource), nil
	})
}
