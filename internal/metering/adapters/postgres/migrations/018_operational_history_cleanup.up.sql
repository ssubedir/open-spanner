CREATE INDEX idx_usage_ingestions_cleanup ON usage_ingestions (created_at, id);
CREATE INDEX idx_usage_prune_runs_cleanup ON usage_prune_runs (created_at, id);
CREATE INDEX idx_consumption_decision_prune_runs_cleanup ON consumption_decision_prune_runs (created_at, id);
CREATE INDEX idx_usage_export_cleanup_runs_cleanup ON usage_export_cleanup_runs (created_at, id);
CREATE INDEX idx_usage_export_jobs_cleanup ON usage_export_jobs (completed_at, id)
	WHERE status = 'completed' AND expired_at IS NOT NULL;
CREATE INDEX idx_alert_delivery_jobs_cleanup ON alert_delivery_jobs (delivered_at, id)
	WHERE status = 'delivered';
CREATE INDEX idx_reconciliation_runs_cleanup ON reconciliation_runs (created_at, id);
CREATE INDEX idx_reconciliation_notifications_cleanup ON reconciliation_notifications (delivered_at, id)
	WHERE status = 'delivered';
CREATE INDEX idx_usage_rollup_runs_cleanup ON usage_rollup_runs (created_at, id);
