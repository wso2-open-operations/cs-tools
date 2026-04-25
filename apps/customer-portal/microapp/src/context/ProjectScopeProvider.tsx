import React from "react";
import FiltersProvider from "./filters/FiltersProvider";

export default function ProjectScopeProvider({ children }: { children: React.ReactNode }) {
  return <FiltersProvider>{children}</FiltersProvider>;
}
