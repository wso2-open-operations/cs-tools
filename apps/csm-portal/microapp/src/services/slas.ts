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

import { queryOptions } from "@tanstack/react-query";
import { SLAS_SEARCH_ENDPOINT } from "@config/endpoints";
import type { SearchTaskSlasResponseDto } from "@src/types";
import { toCaseSla, type CaseSla } from "@src/types";
import apiClient from "./apiClient";

// A case is a "task" upstream — SLAs are searched by task id. ServiceNow data source only.
// openapi.yaml's declared pagination maximums have proven unreliable against the live upstream
// (the comments/search endpoint documents 100 but actually 400s past 50 — see services/cases.ts);
// use the same conservative 50 here rather than trust the doc for this endpoint too.
const getCaseSlas = async (caseId: string): Promise<CaseSla[]> => {
  const { data } = await apiClient.post<SearchTaskSlasResponseDto>(SLAS_SEARCH_ENDPOINT, {
    filters: { taskIds: [caseId] },
    pagination: { limit: 50 },
  });
  return (data.slas ?? []).map(toCaseSla);
};

export const slas = {
  forCase: (caseId: string) =>
    queryOptions({
      queryKey: ["case", caseId, "slas"],
      queryFn: () => getCaseSlas(caseId),
    }),
};
