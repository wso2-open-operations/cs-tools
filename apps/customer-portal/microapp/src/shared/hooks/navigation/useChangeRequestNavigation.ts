import { useNavigate } from "react-router-dom";

import { OUTSTANDING_CHANGE_REQUESTS_TITLE, ROUTES, STATUS_MODE } from "@shared/constants";
import type { ModeType } from "@shared/types";

export const useChangeRequestNavigation = () => {
  const navigate = useNavigate();

  return {
    toOutstandingChangeRequests: () =>
      navigate(ROUTES.change_requests.all, {
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
