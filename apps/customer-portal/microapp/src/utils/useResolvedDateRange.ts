import { useMemo } from "react";
import type { ModeType } from "@root/src/pages/AllItemsPage";

export function useResolvedDateRange(mode?: ModeType) {
  return useMemo(() => {
    if (mode?.type === "status" && mode.status === "resolved") {
      const now = new Date();
      const past = new Date();
      past.setDate(now.getDate() - 30);
      return {
        closedStartDate: past.toISOString().split(".")[0] + "Z",
        closedEndDate: now.toISOString().split(".")[0] + "Z",
      };
    }
    return null;
  }, [mode?.type, (mode as any)?.status]);
}
