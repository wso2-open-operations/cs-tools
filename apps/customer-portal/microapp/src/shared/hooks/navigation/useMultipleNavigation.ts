import { useNavigate } from "react-router-dom";

import { useProject } from "@context/project";

import { useLastMonthRange } from "@features/items/hooks";

import {
  ACTION_REQUIRED_CASE_STATUS_IDS,
  ACTION_REQUIRED_CHANGE_REQUEST_STATUS_IDS,
  ACTION_REQUIRED_ITEMS_TITLE,
  CLOSED_ITEMS_TITLE,
  OUTSTANDING_CASE_STATUS_IDS,
  OUTSTANDING_CHANGE_REQUESTS_STATUS_IDS,
  OUTSTANDING_ITEMS_TITLE,
  OVERVIEW_CASE_TYPES,
  RESOLVED_STATUS_IDS,
} from "@shared/constants";

export const useMultipleNavigation = () => {
  const navigate = useNavigate();
  const { features } = useProject();
  const { start, end } = useLastMonthRange();

  return {
    toClosedItems: () =>
      navigate(
        {
          pathname: "/support/all",
          search: new URLSearchParams([
            ...OVERVIEW_CASE_TYPES.map((type) => ["type", type]),
            ...RESOLVED_STATUS_IDS.map((state) => ["state", String(state)]),
            ...[
              ["startDate", start],
              ["endDate", end],
            ],
          ]).toString(),
        },
        { state: { title: CLOSED_ITEMS_TITLE } },
      ),

    toOutstandingItems: () =>
      navigate(
        {
          pathname: "/support/all",
          search: new URLSearchParams([
            ...OVERVIEW_CASE_TYPES.map((type) => ["type", type]),
            ...OUTSTANDING_CASE_STATUS_IDS.map((state) => ["status", String(state)]),
            ...(features?.hasChangeRequestReadAccess
              ? OUTSTANDING_CHANGE_REQUESTS_STATUS_IDS.map((state) => ["state", String(state)])
              : []),
          ]).toString(),
        },
        { state: { title: OUTSTANDING_ITEMS_TITLE } },
      ),

    toActionRequiredItems: () =>
      navigate(
        {
          pathname: "/support/all",
          search: new URLSearchParams([
            ...OVERVIEW_CASE_TYPES.map((type) => ["type", type]),
            ...ACTION_REQUIRED_CASE_STATUS_IDS.map((state) => ["status", String(state)]),
            ...(features?.hasChangeRequestReadAccess
              ? ACTION_REQUIRED_CHANGE_REQUEST_STATUS_IDS.map((state) => ["state", String(state)])
              : []),
          ]).toString(),
        },
        { state: { title: ACTION_REQUIRED_ITEMS_TITLE } },
      ),
  };
};
