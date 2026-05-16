import type { CaseType } from "@shared/types";
import { GroupAccordion } from "@shared/components/ui/GroupAccordion";
import { Fragment, type ReactNode } from "react";
import { useFilters } from "@features/items/hooks";

export function ItemsListWrapper({ type, children }: { type: CaseType, children: ReactNode }) {
  const { filters } = useFilters(); 

  if (filters.types.length > 1) return <GroupAccordion type={type}>{children}</GroupAccordion>
  return <Fragment>{children}</Fragment>;
}