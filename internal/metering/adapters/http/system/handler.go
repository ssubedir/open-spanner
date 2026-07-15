package system

import (
	"net/http"
	"time"

	"github.com/ssubedir/open-spanner/internal/metering/adapters/http/internal/respond"
	appsystem "github.com/ssubedir/open-spanner/internal/metering/app/system"
)

type Handler struct {
	service appsystem.Service
}

func NewHandler(service appsystem.Service) *Handler {
	return &Handler{service: service}
}

// Stats returns operational system stats.
//
// @Summary Get system stats
// @ID getSystemStats
// @Tags system
// @Produce json
// @Success 200 {object} StatsResponse
// @Failure 500 {object} respond.ErrorResponse
// @Router /v1/system/stats [get]
func (h *Handler) Stats(w http.ResponseWriter, r *http.Request) {
	stats, err := h.service.Stats(r.Context())
	if err != nil {
		respond.ServiceError(w, err)
		return
	}

	respond.JSON(w, http.StatusOK, statsResponseFromResult(stats))
}

func statsResponseFromResult(stats appsystem.StatsResult) StatsResponse {
	var lastPruneRun *LastPruneRunResponse
	if stats.LastPruneRun.ID != "" {
		lastPruneRun = &LastPruneRunResponse{
			ID:        stats.LastPruneRun.ID,
			Deleted:   stats.LastPruneRun.Deleted,
			DryRun:    stats.LastPruneRun.DryRun,
			CreatedAt: stats.LastPruneRun.CreatedAt.Format(time.RFC3339),
		}
	}
	var lastDecisionPruneRun *LastDecisionPruneRunResponse
	if stats.LastDecisionPruneRun.ID != "" {
		lastDecisionPruneRun = &LastDecisionPruneRunResponse{
			ID: stats.LastDecisionPruneRun.ID, Before: stats.LastDecisionPruneRun.Before.Format(time.RFC3339),
			Deleted: stats.LastDecisionPruneRun.Deleted, DryRun: stats.LastDecisionPruneRun.DryRun,
			CreatedAt: stats.LastDecisionPruneRun.CreatedAt.Format(time.RFC3339),
		}
	}

	return StatsResponse{
		Meters:               stats.Meters,
		UsageEvents:          stats.UsageEvents,
		PruneRuns:            stats.PruneRuns,
		LastPruneRun:         lastPruneRun,
		ConsumptionDecisions: stats.ConsumptionDecisions,
		DecisionPruneRuns:    stats.DecisionPruneRuns,
		LastDecisionPruneRun: lastDecisionPruneRun,
	}
}
