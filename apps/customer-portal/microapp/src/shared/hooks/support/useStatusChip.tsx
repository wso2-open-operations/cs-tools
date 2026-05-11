import { useFilters } from "@context/filters";

import { CASE_STATUS_CHIP_COLOR_CONFIG, CASE_TYPES } from "@shared/constants";
import type { CaseType } from "@shared/types";

type ChipData = { label: string; color: string };

export function useStatusChip(type: CaseType, id?: string): ChipData | null {
  const { data, isLoading } = useFilters();
  if (isLoading || !data) return null;

  const items =
    {
      [CASE_TYPES.CHAT]: data.conversationStates,
      [CASE_TYPES.CHANGE_REQUEST]: data.changeRequestStates,
      [CASE_TYPES.DEFAULT]: data.caseStates,
    }[type] ?? data.caseStates;

  return {
    label: items.find((item) => item.id === id)?.label ?? "N/A",
    color: CASE_STATUS_CHIP_COLOR_CONFIG[type]?.[id ?? ""] ?? "default",
  };
}
