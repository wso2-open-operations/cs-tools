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

import axios from "axios";
import { users } from "@root/src/services/users";
import { useQuery } from "@tanstack/react-query";
import { MeContext } from "./MeContext";
import { LoadingFallback } from "@root/src/components/ui/LoadingFallback";
import { AuthorizationFallback } from "@root/src/components/ui";
import { ADMIN_USER_ROLE } from "@root/src/config/constants";
import { getLastVisitedProjectId } from "@root/src/utils/others";
import { projects } from "@root/src/services/projects";
import { useEffect, useMemo, useState } from "react";
import { useProject } from "../project";
import { useNavigate } from "react-router-dom";

export default function MeProvider({ children }: { children: React.ReactNode }) {
  const navigate = useNavigate();
  const { setProjectId } = useProject();
  const lastVisitedProjectId = getLastVisitedProjectId();
  const { data: meData, isLoading: meLoading, error: meError } = useQuery(users.me());
  const { data: projectsData, isLoading: projectsLoading, error: projectsError } = useQuery({ ...projects.all() });
  const { isLoading: isFeaturesLoading, isError: isFeaturesError } = useQuery({
    ...projects.features(lastVisitedProjectId!),
    enabled: !!lastVisitedProjectId,
  });

  const [initialized, setInitialized] = useState(false);

  const lastVisitedProject = useMemo(() => {
    if (!projectsData || !lastVisitedProjectId) return undefined;

    return projectsData.find((p) => p.id === lastVisitedProjectId);
  }, [projectsData, lastVisitedProjectId]);

  useEffect(() => {
    if (initialized) return;

    if (lastVisitedProject) {
      setProjectId(lastVisitedProject.id);
      navigate("/", { replace: true });

      setInitialized(true);
    }
  }, [initialized, lastVisitedProject, setProjectId, navigate]);

  if (meLoading || projectsLoading || isFeaturesLoading || isFeaturesError) return <LoadingFallback />;

  if (axios.isAxiosError(meError) || axios.isAxiosError(projectsError)) return <AuthorizationFallback />;

  if (!meData) return <AuthorizationFallback />;

  return (
    <MeContext.Provider
      value={{
        id: meData.id,
        roles: meData.roles,
        isAdmin: meData.roles.includes(ADMIN_USER_ROLE),
        timezone: meData.timezone,
      }}
    >
      {children}
    </MeContext.Provider>
  );
}
