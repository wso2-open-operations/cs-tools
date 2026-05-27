import { useNavigate } from "react-router-dom";

import {
  CASE_TYPES,
  OUTSTANDING_SERVICE_REQUESTS_STATUS_IDS,
  OUTSTANDING_SERVICE_REQUESTS_TITLE,
} from "@shared/constants";

export const useServiceRequestNavigation = () => {
  const navigate = useNavigate();

  return {
    toOutstandingServiceRequests: () =>
      navigate(
        {
          pathname: "/support/all",
          search: new URLSearchParams([
            ...[["type", CASE_TYPES.SERVICE_REQUEST]],
            ...OUTSTANDING_SERVICE_REQUESTS_STATUS_IDS.map((state) => ["status", String(state)]),
          ]).toString(),
        },

        {
          state: { title: OUTSTANDING_SERVICE_REQUESTS_TITLE },
        },
      ),
  };
};
