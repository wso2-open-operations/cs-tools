import type { CasesFiltersDto } from "@features/cases/types/case.dto";
import { createContext } from "react";

export type FiltersContextType = {
  data?: CasesFiltersDto;
  isLoading: boolean;
};

export const FiltersContext = createContext<FiltersContextType | null>(null);
