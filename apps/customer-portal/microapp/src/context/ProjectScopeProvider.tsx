import type { ReactNode } from "react";
import FiltersProvider from "./filters/FiltersProvider";

export default function ProjectScopeProvider({ children }: { children: ReactNode }) {
  return <FiltersProvider>{children}</FiltersProvider>;
}
