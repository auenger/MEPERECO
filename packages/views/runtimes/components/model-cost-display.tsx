"use client";
import type { ModelCost } from "@silieco/core/types";
import { useWorkspacePaths } from "@silieco/core/paths";
import { useT } from "../../i18n";
import { AppLink } from "../../navigation";
import { formatModelMoney, usePricingCurrency } from "../pricing-currency";
import { formatTokens } from "../utils";

export { sumModelCosts } from "../usage-cost-model";

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
