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

import { useEffect, useMemo } from "react";
import { useNavigate } from "react-router-dom";
import { useSuspenseQuery } from "@tanstack/react-query";
import { projects } from "@features/projects/api/projects.queries";
import { useProject } from "@context/project";

export function useProjectList(search: string) {
  const navigate = useNavigate();
  const { setProjectId } = useProject();
  const { data: projectsData } = useSuspenseQuery(projects.all());

  const filtered = useMemo(() => {
    if (!search) return projectsData;
    const normalizedSearch = search.toLowerCase();
    return projectsData.filter(
      (project) =>
        project.id.toLowerCase().includes(normalizedSearch) ||
        project.name.toLowerCase().includes(normalizedSearch) ||
        project.description?.toLowerCase().includes(normalizedSearch),
    );
  }, [projectsData, search]);

  useEffect(() => {
    if (projectsData.length !== 1) return;
    if (search) return;
    setProjectId(projectsData[0].id);
    navigate("/", { replace: true });
  }, [navigate, projectsData, search, setProjectId]);

  const selectProject = (id: string) => {
    setProjectId(id);
    navigate("/");
  };

  return { projects: filtered, selectProject, isEmpty: filtered.length === 0, isSingle: filtered.length === 1 };
}
