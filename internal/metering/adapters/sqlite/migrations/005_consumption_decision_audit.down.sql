DROP INDEX idx_consumption_decisions_workspace_meter;
DROP INDEX idx_consumption_decisions_workspace_subject;
DROP INDEX idx_consumption_decisions_workspace_audit;
ALTER TABLE consumption_decisions DROP COLUMN state;
ALTER TABLE consumption_decisions DROP COLUMN enforcement;
ALTER TABLE consumption_decisions DROP COLUMN evaluation_failed;
ALTER TABLE consumption_decisions DROP COLUMN accepted;
ALTER TABLE consumption_decisions DROP COLUMN meter_name;
ALTER TABLE consumption_decisions DROP COLUMN subject;
