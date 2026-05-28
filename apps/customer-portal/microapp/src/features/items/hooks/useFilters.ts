import { useMemo } from "react";

import { useLocation, useSearchParams } from "react-router-dom";

import type { CaseType } from "@shared/types";

export interface ListFilterParams {
  types: CaseType[];
  statuses?: string[];
  states?: string[]; // Only exists for Type: Change Requests
  severities?: string[];
  search?: string;
  startDate?: string;
  endDate?: string;
}

export function useFilters() {
  const [searchParams, setSearchParams] = useSearchParams();
  const { state } = useLocation();

  const filters: ListFilterParams = useMemo(
    () => ({
      types: searchParams.getAll("type") as CaseType[],
      statuses: searchParams.getAll("status") || undefined,
      states: searchParams.getAll("state") || undefined, // Only exists for Type: Change Requests
      severities: searchParams.getAll("severity") || undefined,
      search: searchParams.get("search") || undefined,
      startDate: searchParams.get("startDate") || undefined,
      endDate: searchParams.get("endDate") || undefined,
    }),
    [searchParams],
  );

  const patch = (partial: Partial<ListFilterParams>) => {
    setSearchParams(
      (prev) => {
        if (partial.types) {
          prev.delete("type");
          partial.types.forEach((t) => prev.append("type", t));
        }

        if (partial.states) {
          prev.delete("state");
          partial.states.forEach((s) => prev.append("state", s));
        }

        if (partial.statuses) {
          prev.delete("status");
          partial.statuses.forEach((s) => prev.append("status", s));
        }

        if (partial.severities) {
          prev.delete("severity");
          partial.severities.forEach((s) => prev.append("severity", s));
        }

        if (partial.search !== undefined) prev.set("search", partial.search);

        if (partial.startDate !== undefined) prev.set("startDate", partial.startDate);

        if (partial.endDate !== undefined) prev.set("endDate", partial.endDate);

        return prev;
      },
      { replace: true },
    );
  };

  const reset = () => setSearchParams(new URLSearchParams());

  return {
    state: state as { title?: string } | undefined,
    filters,
    set: patch,
    reset,
  };
}
