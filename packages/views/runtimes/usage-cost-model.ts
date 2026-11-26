import type { ModelCost } from "@silieco/core/types";

export interface UsageCostRow {
  pricing?: ModelCost;
  input_tokens?: number;
  output_tokens?: number;
  cache_read_tokens?: number;
  cache_write_tokens?: number;
}

// The workspace pricing API is authoritative. Older servers that omit pricing
// cannot establish a configured rate; do not guess one or imply free usage.
export function modelCostForRow(row: UsageCostRow): ModelCost {
  if (row.pricing) return row.pricing;
  return {
    actual_usd: 0, estimated_usd: 0,
    unknown_tokens: (row.input_tokens ?? 0) + (row.output_tokens ?? 0) + (row.cache_read_tokens ?? 0) + (row.cache_write_tokens ?? 0),
    priced_tokens: 0, estimated_tokens: 0, cache_savings_usd: 0,
    breakdown: { input: 0, output: 0, cache_read: 0, cache_write: 0 },
  };
}

export function sumModelCosts(rows: readonly UsageCostRow[]): ModelCost | undefined {
  if (!rows.length) return undefined;
  return rows.reduce<ModelCost>((a, r) => {
    const p = modelCostForRow(r);
    return {
      actual_usd: a.actual_usd + p.actual_usd,
      estimated_usd: a.estimated_usd + p.estimated_usd,
      unknown_tokens: a.unknown_tokens + p.unknown_tokens,
      priced_tokens: a.priced_tokens + p.priced_tokens,
      estimated_tokens: a.estimated_tokens + p.estimated_tokens,
      cache_savings_usd: a.cache_savings_usd + p.cache_savings_usd,
      breakdown: { input: a.breakdown.input + p.breakdown.input, output: a.breakdown.output + p.breakdown.output, cache_read: a.breakdown.cache_read + p.breakdown.cache_read, cache_write: a.breakdown.cache_write + p.breakdown.cache_write },
    };
  }, modelCostForRow({}));
}
