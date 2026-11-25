import { useQuery } from "@tanstack/react-query";
import { useWorkspaceId } from "@silieco/core/hooks";
import { api } from "@silieco/core/api";
import type { PricingCurrency } from "@silieco/core/types";

export const USD_CURRENCY: PricingCurrency = { currency: "USD", usd_to_cny: 0 };
export const pricingCurrencyKey = (wsId: string) => ["pricing-currency", wsId] as const;
export function usePricingCurrencyQuery() {
  const wsId=useWorkspaceId();
  return useQuery({queryKey:pricingCurrencyKey(wsId),queryFn:()=>api.getWorkspacePricingCurrency(wsId),enabled:!!wsId,staleTime:60_000,retry:false});
}
export function usePricingCurrency() { return usePricingCurrencyQuery().data ?? USD_CURRENCY; }
export function currencyFactor(c: PricingCurrency) { return c.currency === "CNY" && c.usd_to_cny > 0 ? c.usd_to_cny : 1; }
export function currencySymbol(c: PricingCurrency) { return c.currency === "CNY" && c.usd_to_cny > 0 ? "¥" : "$"; }
export function formatModelMoney(usd: number, c: PricingCurrency, rate=false) {
  const amount=usd*currencyFactor(c);
  return currencySymbol(c)+(rate ? Number(amount.toPrecision(12)).toString() : amount>0 && amount<0.01 ? amount.toPrecision(2) : amount.toFixed(2));
}
