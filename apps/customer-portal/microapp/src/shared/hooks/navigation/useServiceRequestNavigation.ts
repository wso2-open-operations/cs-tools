import { useNavigate } from "react-router-dom";

import { CASE_TYPES, OUTSTANDING_SERVICE_REQUESTS_TITLE, ROUTES, STATUS_MODE } from "@shared/constants";
import type { ModeType } from "@shared/types";

export const useServiceRequestNavigation = () => {
  const navigate = useNavigate();

  return {
    toOutstandingServiceRequests: () =>
      navigate(ROUTES[CASE_TYPES.SERVICE_REQUEST].all, {
        state: {
          mode: { type: "status", status: STATUS_MODE.OUTSTANDING } as ModeType,
          title: OUTSTANDING_SERVICE_REQUESTS_TITLE,
        },
      }),
  };
};
