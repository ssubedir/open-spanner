package usage

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/ssubedir/open-spanner/internal/metering/adapters/http/internal/access"
	"github.com/ssubedir/open-spanner/internal/metering/adapters/http/internal/request"
	appusage "github.com/ssubedir/open-spanner/internal/metering/app/usage"
	domainusage "github.com/ssubedir/open-spanner/internal/metering/domain/usage"
)

func (h *Handler) RegisterRoutes(router chi.Router, authorizer access.Authorizer) {
	routes := access.NewRouter(router, authorizer)
	h.registerUsageRoutes(routes)
	h.registerUsageEventRoutes(routes)
	h.registerExportRoutes(routes)
	routes.Get("/usageingestions", h.ListIngestions, access.UsageRead(allUsageResource))
}

func (h *Handler) registerUsageRoutes(routes access.Router) {
	routes.Route("/usages", func(r access.Router) {
		r.Get("/", h.List, access.UsageRead(listQueryUsageResource))
		r.Post("/", h.Create, access.UsageWrite(createUsageResource))
		r.Post("/bulk", h.CreateBulk, access.UsageWrite(bulkUsageResource))
		r.Get("/dimensions", h.ListDimensionValues, access.UsageRead(dimensionValuesResource))
		r.Get("/export", h.Export, access.UsageRead(listQueryUsageResource), access.ExportsRead(listQueryExportResource))
		r.Post("/export", h.ExportSearch, access.UsageRead(searchRequestUsageResource), access.ExportsRead(searchRequestExportResource))
		r.Post("/search", h.Search, access.UsageRead(searchRequestUsageResource))
		r.Post("/breakdowns/search", h.SearchBreakdown, access.UsageRead(breakdownRequestUsageResource))
	})
}

func (h *Handler) registerUsageEventRoutes(routes access.Router) {
	routes.Route("/usageevents", func(r access.Router) {
		r.Get("/", h.ListEvents, access.UsageRead(eventListQueryUsageResource))
		r.Get("/export", h.ExportEvents, access.UsageRead(eventListQueryUsageResource), access.ExportsRead(eventListQueryExportResource))
		r.Post("/export", h.ExportEventsSearch, access.UsageRead(eventSearchRequestUsageResource), access.ExportsRead(eventSearchRequestExportResource))
		r.Post("/prune", h.PruneEvents, access.UsageWrite(allUsageResource))
		r.Get("/prunes", h.ListPruneRuns, access.UsageRead(allUsageResource))
		r.Post("/search", h.SearchEvents, access.UsageRead(eventSearchRequestUsageResource))
	})
}

func (h *Handler) registerExportRoutes(routes access.Router) {
	routes.Route("/exports", func(r access.Router) {
		r.Get("/", h.ListExportJobs, access.ExportsRead(allExportResource))
		r.Post("/", h.CreateExportJob, access.UsageRead(exportJobCreateUsageResource), access.ExportsWrite(exportJobCreateExportResource))
		r.Get("/{id}", h.GetExportJob, access.ExportsRead(h.exportJobResource))
		r.Post("/{id}/cancel", h.CancelExportJob, access.ExportsWrite(h.exportJobResource))
		r.Post("/{id}/retry", h.RetryExportJob, access.ExportsWrite(h.exportJobResource))
		r.Get("/{id}/download", h.DownloadExportJob, access.ExportsRead(h.exportJobResource))
	})
}

var (
	allUsageResource    = access.Static(access.Usage("", ""))
	allExportResource   = access.Static(access.Export(""))
	createUsageResource = access.JSONBodyResource(func(req CreateRequest) (access.Resource, error) {
		return access.Usage(req.Meter, req.Subject), nil
	})
	bulkUsageResource          = access.JSONBody(bulkUsageResources)
	searchRequestUsageResource = access.JSONBodyResource(func(req SearchRequest) (access.Resource, error) {
		query, err := searchListQuery(req)
		if err != nil {
			return access.Resource{}, err
		}
		return access.Usage(query.MeterName, query.Subject), nil
	})
	searchRequestExportResource = access.JSONBodyResource(func(req SearchRequest) (access.Resource, error) {
		query, err := searchListQuery(req)
		if err != nil {
			return access.Resource{}, err
		}
		return access.Export(query.MeterName), nil
	})
	breakdownRequestUsageResource = access.JSONBodyResource(func(req BreakdownRequest) (access.Resource, error) {
		if _, err := request.RequiredTime("from", req.From); err != nil {
			return access.Resource{}, err
		}
		if _, err := request.RequiredTime("to", req.To); err != nil {
			return access.Resource{}, err
		}
		if _, err := filterFromRequest(req.Filter); err != nil {
			return access.Resource{}, err
		}
		return access.Usage(req.Meter, req.Subject), nil
	})
	exportJobCreateUsageResource = access.JSONBodyResource(func(req ExportJobCreateRequest) (access.Resource, error) {
		if _, err := exportJobQueryJSON(req.Query); err != nil {
			return access.Resource{}, err
		}
		return access.Usage(req.Query.Meter, req.Query.Subject), nil
	})
	exportJobCreateExportResource = access.JSONBodyResource(func(req ExportJobCreateRequest) (access.Resource, error) {
		if _, err := exportJobQueryJSON(req.Query); err != nil {
			return access.Resource{}, err
		}
		return access.Export(req.Query.Meter), nil
	})
	eventSearchRequestUsageResource = access.JSONBodyResource(func(req EventSearchRequest) (access.Resource, error) {
		query, err := searchEventListQuery(req)
		if err != nil {
			return access.Resource{}, err
		}
		return access.Usage(query.MeterName, query.Subject), nil
	})
	eventSearchRequestExportResource = access.JSONBodyResource(func(req EventSearchRequest) (access.Resource, error) {
		query, err := searchEventListQuery(req)
		if err != nil {
			return access.Resource{}, err
		}
		return access.Export(query.MeterName), nil
	})
)

func bulkUsageResources(req []CreateRequest) ([]access.Resource, error) {
	seen := map[string]struct{}{}
	resources := make([]access.Resource, 0, len(req))
	for _, item := range req {
		key := item.Meter + "\x00" + item.Subject
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		resources = append(resources, access.Usage(item.Meter, item.Subject))
	}
	return resources, nil
}

func dimensionValuesResource(r *http.Request) ([]access.Resource, error) {
	query := r.URL.Query()
	return access.Resources(access.Usage(query.Get("meter"), query.Get("subject"))), nil
}

func listQueryUsageResource(r *http.Request) ([]access.Resource, error) {
	query, err := listQueryFromRequest(r)
	if err != nil {
		return nil, err
	}
	return access.Resources(access.Usage(query.MeterName, query.Subject)), nil
}

func listQueryExportResource(r *http.Request) ([]access.Resource, error) {
	query, err := listQueryFromRequest(r)
	if err != nil {
		return nil, err
	}
	return access.Resources(access.Export(query.MeterName)), nil
}

func eventListQueryUsageResource(r *http.Request) ([]access.Resource, error) {
	query, err := eventListQueryFromRequest(r)
	if err != nil {
		return nil, err
	}
	return access.Resources(access.Usage(query.MeterName, query.Subject)), nil
}

func eventListQueryExportResource(r *http.Request) ([]access.Resource, error) {
	query, err := eventListQueryFromRequest(r)
	if err != nil {
		return nil, err
	}
	return access.Resources(access.Export(query.MeterName)), nil
}

func (h *Handler) exportJobResource(r *http.Request) ([]access.Resource, error) {
	job, err := h.service.GetExportJob(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		return nil, err
	}
	return access.Resources(access.ExportByID(job.ID, exportJobMeter(job))), nil
}

func listQueryFromRequest(r *http.Request) (appusage.ListQuery, error) {
	query := r.URL.Query()
	limit, err := request.ParseLimit(query.Get("limit"))
	if err != nil {
		return appusage.ListQuery{}, err
	}

	from, err := request.RequiredTime("from", query.Get("from"))
	if err != nil {
		return appusage.ListQuery{}, err
	}
	to, err := request.RequiredTime("to", query.Get("to"))
	if err != nil {
		return appusage.ListQuery{}, err
	}

	return appusage.ListQuery{
		Subject:    query.Get("subject"),
		MeterName:  query.Get("meter"),
		From:       from,
		To:         to,
		BucketSize: domainusage.BucketSize(query.Get("bucket_size")),
		Metadata:   metadataFilters(query),
		GroupBy:    domainusage.SplitGroupByValues(query["group_by"]),
		Limit:      limit,
	}, nil
}

func eventListQueryFromRequest(r *http.Request) (appusage.EventListQuery, error) {
	query := r.URL.Query()
	limit, err := request.ParseLimit(query.Get("limit"))
	if err != nil {
		return appusage.EventListQuery{}, err
	}

	from, err := request.OptionalTime("from", query.Get("from"))
	if err != nil {
		return appusage.EventListQuery{}, err
	}
	to, err := request.OptionalTime("to", query.Get("to"))
	if err != nil {
		return appusage.EventListQuery{}, err
	}

	return appusage.EventListQuery{
		Subject:   query.Get("subject"),
		MeterName: query.Get("meter"),
		From:      from,
		To:        to,
		Limit:     limit,
		Cursor:    query.Get("cursor"),
	}, nil
}
