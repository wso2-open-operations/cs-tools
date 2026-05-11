import { useNavigate } from "react-router-dom";

import { CASE_TYPES, OUTSTANDING_CHANGE_REQUESTS_TITLE, ROUTES, STATUS_MODE } from "@shared/constants";
import type { ModeType } from "@shared/types";

export const useChangeRequestNavigation = () => {
  const navigate = useNavigate();

  return {
    toOutstandingChangeRequests: () =>
      navigate(ROUTES[CASE_TYPES.CHANGE_REQUEST].all, {
        state: {
          mode: {
            type: "status",
            status: STATUS_MODE.OUTSTANDING,
            title: OUTSTANDING_CHANGE_REQUESTS_TITLE,
          } as ModeType,
        },
      }),
  };
};
