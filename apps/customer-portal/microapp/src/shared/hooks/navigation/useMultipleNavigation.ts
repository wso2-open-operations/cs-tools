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
