-- Diagnostic snapshots, not acceptance evidence. Historical results stay intact.
CREATE TABLE workflow_stage_readiness (
    workflow_instance_id uuid PRIMARY KEY REFERENCES workflow_instance(id) ON DELETE CASCADE,
    workspace_id uuid NOT NULL REFERENCES workspace(id) ON DELETE CASCADE,
    stage_id uuid NOT NULL REFERENCES workflow_instance_stage(id) ON DELETE CASCADE,
    state_token text NOT NULL,
    evaluated_at timestamptz NOT NULL,
    evidence_hash text NOT NULL,
    evaluation jsonb NOT NULL CHECK (jsonb_typeof(evaluation) = 'object')
);
