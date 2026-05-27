import { useFilters } from "@context/filters";

import { CASE_TYPES, IMPACT_CHIP_COLOR_CONFIG, PRIORITY_CHIP_COLOR_CONFIG } from "@shared/constants";
import type { CaseType } from "@shared/types";

type ChipData = { label: string; color: string };

export function usePriorityChip(type: CaseType, id?: string): ChipData | null {
  const { data, isLoading } = useFilters();
  if (isLoading || !data) return null;

  const isTypeChangeRequest = type === CASE_TYPES.CHANGE_REQUEST;
  const items = isTypeChangeRequest ? data.changeRequestImpacts : data.severities;
  const colors = isTypeChangeRequest ? IMPACT_CHIP_COLOR_CONFIG : PRIORITY_CHIP_COLOR_CONFIG;

  return {
    label: items.find((item) => item.id === id)?.label ?? "N/A",
    color: colors[id ?? ""] ?? "default",
  };
}
