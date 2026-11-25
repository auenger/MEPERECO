"use client";

import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@silieco/ui/components/ui/select";

interface Option { value: string; label: string }
// Base UI uses nonempty item values. Prefix every option, including the empty
// filter value, so it cannot collide with a repository name or project ID.
export function GitSelect({ label, value, options, onChange, disabled, className }: {
  label: string; value: string; options: Option[]; onChange: (value: string) => void; disabled?: boolean; className?: string;
}) {
  const items = options.map(option => ({ value: `git:${option.value}`, label: option.label }));
  return <Select items={items} value={`git:${value}`} onValueChange={next => { if (next != null) onChange(next.slice(4)); }} disabled={disabled}>
    <SelectTrigger aria-label={label} className={`min-w-0 max-w-full ${className ?? ""}`}><SelectValue className="min-w-0 truncate" /></SelectTrigger>
    <SelectContent align="start" alignItemWithTrigger={false} className="max-w-[min(420px,calc(100vw-2rem))]">
      {items.map(item => <SelectItem key={item.value} value={item.value}><span className="min-w-0 truncate" title={item.label}>{item.label}</span></SelectItem>)}
    </SelectContent>
  </Select>;
}
