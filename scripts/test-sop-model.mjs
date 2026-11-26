import assert from "node:assert/strict";
import test from "node:test";
import { stagesFromWorkflow, serializeSopStages, cloneStages, SOP_TEMPLATES } from "../packages/views/workflows/components/sop-designer-model.ts";

test("SOP draft round-trips stable keys, legacy policy and contract metadata", () => {
  const stage = {
    stable_key: "s1", name: "Review", description: "detail", execution_mode: "manual", executor_id: null,
    completion_rule: { schema_version: "meperstudio.completion/v1", type: "all_tasks_terminal", evaluation_mode: "scheduled", schedule: "0 18 * * *", timezone: "Asia/Shanghai", allowed_task_statuses: ["todo", "done"] },
    input_spec: { default_artifact: "input", extra: "value" }, output_spec: { default_artifact: "output", extra: "value" }, input_bindings: {},
    gate: { type: "human", decider_type: "human", decider: "member-1", require_human: true, human_decider: "member-1", custom: "retained" },
    required_skills: ["skill-1"], allowed_mcp_server_ids: ["mcp-1"], rollback_stage_key: null,
    rework_policy: { max_reworks: 2 }, retry_policy: { max_attempts: 3 },
  };
  const drafts = stagesFromWorkflow({ current_version: { stages: [stage] } });
  const result = serializeSopStages([{ ...drafts[0], name: "New title" }], []);
  assert.equal(result[0].name, "New title");
  for (const key of ["stable_key", "completion_rule", "input_spec", "output_spec", "gate", "required_skills", "allowed_mcp_server_ids", "rework_policy", "retry_policy"]) assert.deepEqual(result[0][key], stage[key], key);
});
test("preset stages have unique stable keys and rollback references", () => {
  for (const template of SOP_TEMPLATES) {
    const drafts = cloneStages(template);
    const result = serializeSopStages(drafts, []);
    assert.equal(new Set(result.map(s => s.stable_key)).size, result.length);
    for (let i = 1; i < result.length; i++) assert.equal(result[i].rollback_stage_key, result[i-1].stable_key);
  }
});
