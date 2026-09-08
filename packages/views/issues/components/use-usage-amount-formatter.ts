"use client";

import { useT } from "../../i18n";
import { formatUsd } from "../../runtimes/utils";

export function useUsageAmountFormatter() {
  const { t } = useT("issues");
  return (amount: number, complete: boolean) => {
    if (complete) return formatUsd(amount);
    if (amount === 0) return t(($) => $.usage_detail.amount_unknown);
    return t(($) => $.usage_detail.amount_partial, { amount: formatUsd(amount) });
  };
}
