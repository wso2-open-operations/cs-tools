import { useCallback, useState } from "react";

import { useSearchParams } from "react-router-dom";

import { CASE_TYPES } from "@root/src/shared/constants";

import type { CaseType } from "@shared/types";

const TAB_QUERY_PARAM_KEY = "tab";

export function useActiveTab() {
  const [searchParams, setSearchParams] = useSearchParams();

  const tabQueryParam = searchParams.get(TAB_QUERY_PARAM_KEY);

  const [tab, setTab] = useState<CaseType>(() => {
    return Object.values(CASE_TYPES).includes(tabQueryParam as CaseType)
      ? (tabQueryParam as CaseType)
      : CASE_TYPES.DEFAULT;
  });

  const setCaseTab = useCallback(
    (value: CaseType) => {
      setTab(value);

      setSearchParams(
        (prev) => {
          const next = new URLSearchParams(prev);
          next.set(TAB_QUERY_PARAM_KEY, value);
          return next;
        },
        { replace: true },
      );
    },
    [setSearchParams],
  );

  return { tab, setTab: setCaseTab };
}
