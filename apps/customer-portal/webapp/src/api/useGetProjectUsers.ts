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

import { useQuery } from "@tanstack/react-query";
import type { UseQueryResult } from "@tanstack/react-query";
import { mockProjectUsers } from "@models/mockData";
import type { MockProjectUser } from "@models/mockData";
import { useMockConfig } from "@providers/MockConfigProvider";
import { ApiQueryKeys } from "@constants/apiConstants";

/**
 * Custom hook to fetch project users.
 *
 * @param {string} projectId - The project ID.
 * @returns {UseQueryResult<MockProjectUser[]>} The query result containing project users.
 */
export default function useGetProjectUsers(
    projectId: string,
): UseQueryResult<MockProjectUser[]> {
    const { isMockEnabled } = useMockConfig();

    return useQuery<MockProjectUser[]>({
        queryKey: [ApiQueryKeys.PROJECT_USERS, projectId],
        queryFn: async () => {
            if (isMockEnabled) {
                return mockProjectUsers;
            }

            // TODO: Implement API call when endpoint is available
            return [];
        },
        enabled: !!projectId,
    });
}
