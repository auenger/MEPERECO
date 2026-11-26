ALTER TABLE workflow_stage ALTER COLUMN completion_rule SET DEFAULT '{"type":"all_runs_terminal"}'::jsonb;
ALTER TABLE workflow_instance_stage ALTER COLUMN completion_rule SET DEFAULT '{"type":"all_runs_terminal"}'::jsonb;
