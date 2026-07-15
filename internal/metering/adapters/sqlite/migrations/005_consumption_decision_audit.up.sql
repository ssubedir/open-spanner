ALTER TABLE consumption_decisions ADD COLUMN subject TEXT NOT NULL DEFAULT '';
ALTER TABLE consumption_decisions ADD COLUMN meter_name TEXT NOT NULL DEFAULT '';
ALTER TABLE consumption_decisions ADD COLUMN accepted INTEGER NOT NULL DEFAULT 0 CHECK (accepted IN (0, 1));
ALTER TABLE consumption_decisions ADD COLUMN evaluation_failed INTEGER NOT NULL DEFAULT 0 CHECK (evaluation_failed IN (0, 1));
ALTER TABLE consumption_decisions ADD COLUMN enforcement TEXT NOT NULL DEFAULT '';
ALTER TABLE consumption_decisions ADD COLUMN state TEXT NOT NULL DEFAULT '';

UPDATE consumption_decisions SET
	subject = COALESCE(json_extract(response, '$.Quota.Subject'), ''),
	meter_name = COALESCE(json_extract(response, '$.Quota.MeterName'), ''),
	accepted = COALESCE(json_extract(response, '$.Accepted'), 0),
	evaluation_failed = COALESCE(json_extract(response, '$.EvaluationFailed'), 0),
	enforcement = COALESCE(json_extract(response, '$.Quota.Enforcement'), ''),
	state = COALESCE(json_extract(response, '$.Quota.State'), '');

CREATE INDEX idx_consumption_decisions_workspace_audit
	ON consumption_decisions (workspace_id, created_at DESC, idempotency_key DESC);
CREATE INDEX idx_consumption_decisions_workspace_subject
	ON consumption_decisions (workspace_id, subject, created_at DESC);
CREATE INDEX idx_consumption_decisions_workspace_meter
	ON consumption_decisions (workspace_id, meter_name, created_at DESC);
