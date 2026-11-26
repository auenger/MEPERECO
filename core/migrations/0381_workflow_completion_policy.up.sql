ALTER TABLE workflow_stage ALTER COLUMN completion_rule SET DEFAULT '{"type":"all_tasks_terminal","evaluation_mode":"manual"}'::jsonb;
ALTER TABLE workflow_instance_stage ALTER COLUMN completion_rule SET DEFAULT '{"type":"all_tasks_terminal","evaluation_mode":"manual"}'::jsonb;
