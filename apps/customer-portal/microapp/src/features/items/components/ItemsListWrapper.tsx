import { Fragment, type ReactNode } from "react";

import { useFilters } from "@features/items/hooks";

import { GroupAccordion } from "@shared/components/ui/GroupAccordion";

import type { CaseType } from "@shared/types";

export function ItemsListWrapper({ type, count, children }: { type: CaseType; count?: number; children: ReactNode }) {
  const { filters } = useFilters();

  if (filters.types.length > 1)
    return (
      <GroupAccordion type={type} count={count}>
        {children}
      </GroupAccordion>
    );
  return <Fragment>{children}</Fragment>;
}
