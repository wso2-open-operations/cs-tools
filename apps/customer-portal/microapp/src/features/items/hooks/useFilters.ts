import type { CaseType } from "@shared/types";
import { useSearchParams } from "react-router-dom";

export interface ListFilterParams {
    types: CaseType[];
    states?: string[];
    severities?: string[];
    search?: string;
}

export function useFilters() {
  const [searchParams, setSearchParams] = useSearchParams();

  const filters: ListFilterParams = {
    types: searchParams.getAll("type") as CaseType[],
    states: searchParams.getAll("state") || undefined,
    severities: searchParams.getAll("severity") || undefined,
    search: searchParams.get("search") ?? "",
  };

  const patch = (partial: Partial<ListFilterParams>) => {
    setSearchParams(prev => {
      if (partial.types) { prev.delete("type"); partial.types.forEach(t => prev.append("type", t)); }
      if (partial.states) { prev.delete("state"); partial.states.forEach(s => prev.append("state", s)); }
      if (partial.severities) { prev.delete("severity"); partial.severities.forEach(s => prev.append("severity", s)); }
      if (partial.search !== undefined) prev.set("search", partial.search);
      return prev;
    });
  };

  const reset = () => setSearchParams(new URLSearchParams());

  return {
    filters,
    set: patch,
    reset
  };
}