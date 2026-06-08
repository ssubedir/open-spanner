package bootstrap

import (
	"github.com/go-chi/chi/v5"

	httpmeter "open-spanner/internal/metering/adapters/http/meter"
	httpusage "open-spanner/internal/metering/adapters/http/usage"
	"open-spanner/internal/metering/adapters/memory"
	appmeter "open-spanner/internal/metering/app/meter"
	appusage "open-spanner/internal/metering/app/usage"
)

func RegisterRoutes(router chi.Router) {
	store := memory.NewStore()
	meterRepo := memory.NewMeterRepository(store)
	usageRepo := memory.NewUsageRepository(store)
	meterService := appmeter.NewService(meterRepo)
	usageService := appusage.NewService(meterRepo, usageRepo)

	router.Route("/v1", func(r chi.Router) {
		httpmeter.NewHandler(meterService).RegisterRoutes(r)
		httpusage.NewHandler(usageService).RegisterRoutes(r)
	})
}
