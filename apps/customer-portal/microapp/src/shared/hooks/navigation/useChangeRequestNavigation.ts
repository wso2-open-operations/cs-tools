import { useNavigate } from "react-router-dom";

import {
  CASE_TYPES,
  OUTSTANDING_CHANGE_REQUESTS_STATUS_IDS,
  OUTSTANDING_CHANGE_REQUESTS_TITLE,
} from "@shared/constants";

export const useChangeRequestNavigation = () => {
  const navigate = useNavigate();

  return {
    toOutstandingChangeRequests: () =>
      navigate(
        {
          pathname: "/support/all",
          search: new URLSearchParams([
            ...[["type", CASE_TYPES.CHANGE_REQUEST]],
            ...OUTSTANDING_CHANGE_REQUESTS_STATUS_IDS.map((state) => ["state", String(state)]),
          ]).toString(),
        },
        {
          state: { title: OUTSTANDING_CHANGE_REQUESTS_TITLE },
        },
      ),
  };
};
