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

import { useMemo } from "react";
import { useQuery, useSuspenseQuery } from "@tanstack/react-query";
import { users } from "@features/users/api/users.queries";
import { useProject } from "@context/project";

export function useUserMetrics(projectId: string) {
  const { data } = useQuery(users.all(projectId));

  return {
    total: data?.length,
    registered: data?.filter((u) => u.status === "registered").length,
    invited: data?.filter((u) => u.status === "invited").length,
    admins: data?.filter((u) => u.roles.includes("Admin")).length,
  };
}

export function useUserList(search: string) {
  const { projectId } = useProject();
  const { data: usersData } = useSuspenseQuery(users.all(projectId!));

  const filtered = useMemo(() => {
    if (!search) return usersData;
    const normalizedSearch = search.toLowerCase();
    return usersData.filter(
      (user) =>
        user.firstName.toLowerCase().includes(normalizedSearch) ||
        user.lastName.toLowerCase().includes(normalizedSearch),
    );
  }, [usersData, search]);

  return { users: filtered };
}
