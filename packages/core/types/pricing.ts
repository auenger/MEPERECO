export interface ModelRate { input: number; output: number; cache_read: number; cache_write: number }
export interface ModelCost { actual_usd: number; estimated_usd: number; unknown_tokens: number; priced_tokens: number; estimated_tokens: number; cache_savings_usd: number; breakdown: ModelRate }
export type ModelPricingMode = "default" | "custom" | "alias" | "free";
export type CurrencyCode = "USD" | "CNY";
export interface PricingCurrency { currency: CurrencyCode; usd_to_cny: number }
export interface ModelPricingRule { rate_currency?: CurrencyCode; usd_to_cny?: number; provider: string; model: string; mode: ModelPricingMode; reference_key?: string; rates?: ModelRate }
export interface ModelPricingEntry extends ModelPricingRule { source: "reference" | "custom" | "alias" | "free" | "unpriced"; effective_rates: ModelRate | null; tokens: number; actual_usd: number }
export interface ReferenceRate extends ModelRate { key: string; provider: string; model: string }
export interface WorkspaceModelPricing { entries: ModelPricingEntry[]; reference_rates: ReferenceRate[]; can_manage: boolean }
