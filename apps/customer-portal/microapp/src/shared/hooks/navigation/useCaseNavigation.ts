import { useNavigate } from "react-router-dom";

import { useProject } from "@root/src/context/project";

import { OUTSTANDING_CASES_BY_SEVERITY_TITLE, ROUTES } from "@shared/constants";
import type { ModeType } from "@shared/types";

export const useCaseNavigation = () => {
  const navigate = useNavigate();
  const { noveraEnabled } = useProject();

  return {
    toBySeverity: (id: string | number, label: string) =>
      navigate(ROUTES.cases.all, {
        state: { mode: { type: "severity", id, title: OUTSTANDING_CASES_BY_SEVERITY_TITLE(label) } as ModeType },
      }),

    toCaseCreate: () => navigate(noveraEnabled ? ROUTES.chat : ROUTES.cases.create),
  };
};
