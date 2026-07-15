ALTER TABLE plan_limits
	ADD COLUMN enforcement TEXT NOT NULL DEFAULT 'advisory'
		CHECK (enforcement IN ('advisory', 'hard'));

ALTER TABLE plan_limits
	ADD COLUMN failure_policy TEXT NOT NULL DEFAULT 'fail_open'
		CHECK (failure_policy IN ('fail_open', 'fail_closed'));
