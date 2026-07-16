package sqlite

import (
	"context"

	appauth "github.com/ssubedir/open-spanner/internal/auth"
	appsystem "github.com/ssubedir/open-spanner/internal/metering/app/system"
)

func (r *SystemRepository) FindIngestionSafety(ctx context.Context) (appsystem.IngestionSafety, error) {
	workspaceID, err := appauth.RequireWorkspaceID(ctx)
	if err != nil {
		return appsystem.IngestionSafety{}, err
	}
	var result appsystem.IngestionSafety
	err = r.store.QueryRowContext(ctx, `SELECT
		COALESCE((SELECT SUM(accepted) FROM usage_ingestions WHERE workspace_id = ?), 0),
		COALESCE((SELECT SUM(failed) FROM usage_ingestions WHERE workspace_id = ?), 0),
		COALESCE((SELECT ingestion_throttled FROM workspace_stats WHERE workspace_id = ?), 0)`, workspaceID, workspaceID, workspaceID).
		Scan(&result.AcceptedEvents, &result.RejectedEvents, &result.ThrottledEvents)
	return result, err
}
