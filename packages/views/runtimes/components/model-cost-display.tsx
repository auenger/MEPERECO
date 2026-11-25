"use client";
import type { ModelCost } from "@silieco/core/types";
import { useWorkspacePaths } from "@silieco/core/paths";
import { useT } from "../../i18n";
import { AppLink } from "../../navigation";
import { formatModelMoney, usePricingCurrency } from "../pricing-currency";
import { formatTokens } from "../utils";

export function sumModelCosts(rows: readonly {pricing?: ModelCost}[]): ModelCost | undefined {
 if (!rows.length || rows.some(r=>!r.pricing)) return;
 return rows.reduce<ModelCost>((a,r)=>{const p=r.pricing!;return {actual_usd:a.actual_usd+p.actual_usd,estimated_usd:a.estimated_usd+p.estimated_usd,unknown_tokens:a.unknown_tokens+p.unknown_tokens,priced_tokens:a.priced_tokens+p.priced_tokens,estimated_tokens:a.estimated_tokens+p.estimated_tokens,cache_savings_usd:a.cache_savings_usd+p.cache_savings_usd,breakdown:{input:a.breakdown.input+p.breakdown.input,output:a.breakdown.output+p.breakdown.output,cache_read:a.breakdown.cache_read+p.breakdown.cache_read,cache_write:a.breakdown.cache_write+p.breakdown.cache_write}}},{actual_usd:0,estimated_usd:0,unknown_tokens:0,priced_tokens:0,estimated_tokens:0,cache_savings_usd:0,breakdown:{input:0,output:0,cache_read:0,cache_write:0}});
}

export function ModelCostValue({cost}: {cost:ModelCost}){
 const {t}=useT("usage");const currency=usePricingCurrency();const known=cost.actual_usd+cost.estimated_usd;
 if(cost.unknown_tokens>0&&cost.priced_tokens===0&&known===0)return <span className="text-warning">{t($=>$.pricing.unpriced)}</span>;
 return <span className="tabular-nums">{cost.unknown_tokens>0?"≥ ":""}{formatModelMoney(known,currency)}</span>;
}
export function ModelCostHint({cost}: {cost:ModelCost}){
 const {t}=useT("usage");const currency=usePricingCurrency();return <span className="text-caption text-muted-foreground">{cost.actual_usd>0&&<span>{t($=>$.pricing.actual)} {formatModelMoney(cost.actual_usd,currency)} · </span>}{cost.estimated_tokens>0&&<span>{t($=>$.pricing.estimated)} {formatModelMoney(cost.estimated_usd,currency)} · </span>}{cost.unknown_tokens>0?<span className="text-warning">{formatTokens(cost.unknown_tokens)} {t($=>$.pricing.unknown_tokens)}</span>:<span>{t($=>$.pricing.covered)}</span>}</span>;
}
export function ModelPricingNotice({cost}: {cost:ModelCost}){
 const {t}=useT("usage");const p=useWorkspacePaths();return <div className={`flex flex-wrap items-center justify-between gap-2 rounded-lg border px-3 py-2 text-caption ${cost.unknown_tokens>0?"border-warning/30 bg-warning/10":"bg-muted/20"}`}><div><ModelCostHint cost={cost}/>{cost.unknown_tokens>0&&<p className="mt-1 text-muted-foreground">{t($=>$.pricing.notice)}</p>}</div><AppLink href={p.settings()+"?tab=model-pricing"} className="shrink-0 rounded-md border bg-background px-3 py-1.5 hover:bg-accent">{t($=>$.pricing.settings)}</AppLink></div>;
}

export function AggregatedCostValue({cost,tokens,unknownTokens}: {cost:number;tokens:number;unknownTokens?:number}) {
 const {t}=useT("usage");const currency=usePricingCurrency();return unknownTokens && unknownTokens>=tokens && cost===0 ? <span className="text-warning" title={t($=>$.pricing.notice)}>{t($=>$.pricing.unpriced)}</span>:<span title={unknownTokens?t($=>$.pricing.notice):undefined}>{unknownTokens?"≥ ":""}{formatModelMoney(cost,currency)}</span>;
}
