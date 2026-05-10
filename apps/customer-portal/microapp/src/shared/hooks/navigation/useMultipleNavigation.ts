import { useNavigate } from "react-router-dom";

import {
  ACTION_REQUIRED_ITEMS_TITLE,
  CLOSED_ITEMS_TITLE,
  OUTSTANDING_ITEMS_TITLE,
  ROUTES,
  STATUS_MODE,
} from "@shared/constants";
import type { ModeType } from "@shared/types";

export const useMultipleNavigation = () => {
  const navigate = useNavigate();

  return {
    toClosedItems: () =>
      navigate(ROUTES.multiple.all, {
        state: { mode: { type: "status", status: STATUS_MODE.RESOLVED } as ModeType, title: CLOSED_ITEMS_TITLE },
      }),

    toOutstandingItems: () =>
      navigate(ROUTES.multiple.all, {
        state: {
          mode: { type: "status", status: STATUS_MODE.OUTSTANDING } as ModeType,
          title: OUTSTANDING_ITEMS_TITLE,
        },
      }),

    toActionRequiredItems: () =>
      navigate(ROUTES.multiple.all, {
        state: {
          mode: { type: "status", status: STATUS_MODE.ACTION_REQUIRED, title: ACTION_REQUIRED_ITEMS_TITLE } as ModeType,
        },
      }),
  };
};
