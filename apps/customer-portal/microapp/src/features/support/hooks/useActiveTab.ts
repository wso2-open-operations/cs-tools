// Copyright (c) 2026 WSO2 LLC. (https://www.wso2.com).
//
// WSO2 LLC. licenses this file to you under the Apache License,
// Version 2.0 (the "License"); you may not use this file except
// in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.
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
