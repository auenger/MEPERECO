import assert from 'node:assert/strict';
import test from 'node:test';
import { modelCostForRow, sumModelCosts } from '../packages/views/runtimes/usage-cost-model.ts';

test('missing workspace pricing is unknown, not free or a guessed reference rate', () => {
  const row = { input_tokens: 10, output_tokens: 20, cache_read_tokens: 30, cache_write_tokens: 40 };
  const price = modelCostForRow(row);
  assert.equal(price.unknown_tokens, 100);
  assert.equal(price.estimated_usd, 0);
  assert.equal(price.priced_tokens, 0);
});
test('mixed configured and unconfigured models keep both known cost and unknown tokens', () => {
  const configured = { pricing: { ...modelCostForRow({}), estimated_usd: 2, estimated_tokens: 100, priced_tokens: 100, breakdown: { input: 2, output: 0, cache_read: 0, cache_write: 0 } } };
  const mixed = sumModelCosts([configured, { input_tokens: 50 }]);
  assert.equal(mixed.estimated_usd, 2);
  assert.equal(mixed.unknown_tokens, 50);
  assert.equal(mixed.priced_tokens, 100);
  const free = sumModelCosts([{ pricing: { ...modelCostForRow({}), priced_tokens: 100, estimated_tokens: 100 } }]);
  assert.equal(free.unknown_tokens, 0);
  assert.equal(free.estimated_usd, 0);
});
