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

// GET /products, GET /abt-teams — bare string[] lookups used by
// this domain's filter dropdowns. Small, harmless duplication if another
// SPL domain also needs these (per the merge plan's own guidance).
import { useQuery, type UseQueryResult } from "@tanstack/react-query";
import { useBackendApi } from "@api/backend/client";

export function useProducts(): UseQueryResult<string[], Error> {
  const backendApi = useBackendApi();
  return useQuery<string[], Error>({
    queryKey: ["spl-products"],
    queryFn: async () => (await backendApi.get<string[]>("/products")) ?? [],
    staleTime: 5 * 60 * 1000,
  });
}

export function useAbtTeams(): UseQueryResult<string[], Error> {
  const backendApi = useBackendApi();
  return useQuery<string[], Error>({
    queryKey: ["spl-abt-teams"],
    queryFn: async () => (await backendApi.get<string[]>("/abt-teams")) ?? [],
    staleTime: 5 * 60 * 1000,
  });
}
