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

import { useState } from "react";
import { ProjectContext } from "./ProjectContext";
import { useQuery } from "@tanstack/react-query";
import { projects } from "src/services/projects";
import { setLastVisitedProjectId } from "@root/src/utils/others";

export default function ProjectProvider({ children }: { children: React.ReactNode }) {
  const [projectId, setProjectId] = useState<string | null>(null);

  const { data } = useQuery({
    ...projects.get(projectId!),
    enabled: !!projectId,
  });

  const { data: features } = useQuery({
    ...projects.features(projectId!),
    enabled: !!projectId,
  });

  const setAndStoreProjectId = (id: string | null) => {
    setProjectId(id);
    setLastVisitedProjectId(id); // local storage
  };

  return (
    <ProjectContext.Provider
      value={{
        projectId,
        noveraEnabled: data?.agentEnabled ?? false,
        kbReferencesEnabled: data?.kbReferencesEnabled ?? false,
        setProjectId: setAndStoreProjectId,
        features,
        projectTypeId: data?.typeId,
        type: data?.type,
      }}
    >
      {children}
    </ProjectContext.Provider>
  );
}
