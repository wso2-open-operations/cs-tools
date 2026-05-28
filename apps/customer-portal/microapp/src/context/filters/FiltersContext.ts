import { createContext } from "react";

import type { CasesFiltersDto } from "@features/case-types/cases/types/case.dto";

export type FiltersContextType = {
  data?: CasesFiltersDto;
  isLoading: boolean;
};

export const FiltersContext = createContext<FiltersContextType | null>(null);
