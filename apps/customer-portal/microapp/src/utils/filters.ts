import type { ModeType, OfStatusModeType } from "../pages/AllItemsPage";
import type { EntityReference } from "../types";

const ALLOWED_STATUS_FILTERS: Record<OfStatusModeType["status"], number[]> = {
  action_required: [18, 6],
  outstanding: [1, 10, 6, 1006],
  resolved: [3],
};

export function getAllowedFilters(filters: EntityReference[], mode?: ModeType) {
  if (!mode) return filters;

  switch (mode.type) {
    case "status": {
      const allowed = ALLOWED_STATUS_FILTERS[mode.status];
      if (!allowed) return filters;
      return filters.filter((filter) => allowed.includes(Number(filter.id)));
    }

    default:
      return filters;
  }
}
