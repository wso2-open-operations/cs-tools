import { useNavigate } from "react-router-dom";

import { useProject } from "@context/project";

import type { Case, CaseClassificationResponseDto } from "@features/case-types/cases/types";
import type { BubbleProps } from "@features/case-types/conversations/components";

import { CASE_TYPES, OUTSTANDING_CASES_BY_SEVERITY_TITLE, ROUTES } from "@shared/constants";

export const useCaseNavigation = () => {
  const navigate = useNavigate();
  const { noveraEnabled } = useProject();

  return {
    toBySeverity: (id: string | number, label: string) =>
      navigate(
        {
          pathname: "/support/all",
          search: new URLSearchParams([
            ["type", CASE_TYPES.DEFAULT],
            ["severity", String(id)],
          ]).toString(),
        },
        { state: { title: OUTSTANDING_CASES_BY_SEVERITY_TITLE(label) } },
      ),

    toCaseCreate: () => navigate(noveraEnabled ? ROUTES[CASE_TYPES.CHAT].create : ROUTES[CASE_TYPES.DEFAULT].create),
    toRelativeCaseCreate: (data: Case) => navigate(ROUTES[CASE_TYPES.DEFAULT].create, { state: { case: data } }),
    toClassifiedCaseCreate: (messages: BubbleProps[], classifications: CaseClassificationResponseDto) =>
      navigate(ROUTES[CASE_TYPES.DEFAULT].create, { state: { messages, classifications } }),
  };
};
