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

import { useQuery, type UseQueryResult } from "@tanstack/react-query";
import { getCaseCreationMetadata } from "@/models/mockFunctions";
import { useLogger } from "@/hooks/useLogger";
import { ApiQueryKeys, API_MOCK_DELAY } from "@/constants/apiConstants";
import type { CaseCreationMetadata } from "@/models/mockData";

/**
 * Custom hook to fetch case creation metadata.
 *
 * @returns {UseQueryResult<CaseCreationMetadata, Error>} The query result object.
 */
export function useGetCaseCreationDetails(): UseQueryResult<
  CaseCreationMetadata,
  Error
> {
  const logger = useLogger();

  return useQuery<CaseCreationMetadata, Error>({
    queryKey: [ApiQueryKeys.CASE_CREATION_METADATA],
    queryFn: async (): Promise<CaseCreationMetadata> => {
      logger.debug("Fetching case creation metadata");

      // Mock behavior: simulate network latency for the in-memory mock data.
      await new Promise((resolve) => setTimeout(resolve, API_MOCK_DELAY));

      const metadata: CaseCreationMetadata = getCaseCreationMetadata();

      logger.debug("Case creation metadata fetched successfully", metadata);

      return metadata;
    },
    staleTime: 60 * 60 * 1000,
  });
}
