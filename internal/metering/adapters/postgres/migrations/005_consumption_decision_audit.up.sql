ALTER TABLE consumption_decisions
	ADD COLUMN subject TEXT NOT NULL DEFAULT '',
	ADD COLUMN meter_name TEXT NOT NULL DEFAULT '',
	ADD COLUMN accepted BOOLEAN NOT NULL DEFAULT FALSE,
	ADD COLUMN evaluation_failed BOOLEAN NOT NULL DEFAULT FALSE,
	ADD COLUMN enforcement TEXT NOT NULL DEFAULT '',
	ADD COLUMN state TEXT NOT NULL DEFAULT '';

UPDATE consumption_decisions SET
	subject = COALESCE(response->'Quota'->>'Subject', ''),
	meter_name = COALESCE(response->'Quota'->>'MeterName', ''),
	accepted = COALESCE((response->>'Accepted')::boolean, FALSE),
	evaluation_failed = COALESCE((response->>'EvaluationFailed')::boolean, FALSE),
	enforcement = COALESCE(response->'Quota'->>'Enforcement', ''),
	state = COALESCE(response->'Quota'->>'State', '');

CREATE INDEX idx_consumption_decisions_workspace_audit
	ON consumption_decisions (workspace_id, created_at DESC, idempotency_key DESC);
CREATE INDEX idx_consumption_decisions_workspace_subject
	ON consumption_decisions (workspace_id, subject, created_at DESC);
CREATE INDEX idx_consumption_decisions_workspace_meter
	ON consumption_decisions (workspace_id, meter_name, created_at DESC);
