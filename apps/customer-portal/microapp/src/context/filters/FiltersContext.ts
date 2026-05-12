import type { CasesFiltersDto } from "@root/src/types";
import { createContext } from "react";

export type FiltersContextType = {
  data?: CasesFiltersDto;
  isLoading: boolean;
};

export const FiltersContext = createContext<FiltersContextType | null>(null);
