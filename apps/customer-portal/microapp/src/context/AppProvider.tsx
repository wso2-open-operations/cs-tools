import React, { useCallback, useEffect, useState } from "react";

import { useQueries } from "@tanstack/react-query";

import { FiltersContext } from "@context/filters";
import { MeContext } from "@context/me";
import { ProjectContext } from "@context/project";

import { cases } from "@features/case-types/cases/api/cases.queries";
import { projects } from "@features/projects/api/projects.queries";
import { users } from "@features/users/api/users.queries";

import { AuthorizationFallback, LoadingFallback } from "@shared/components/ui";

import { ADMIN_ROLE_ID } from "@shared/constants";
import { getLastVisitedProjectId, setLastVisitedProjectId } from "@shared/utils";

export function AppProvider({ children }: { children: React.ReactNode }) {
  const lastVisitedProjectId = getLastVisitedProjectId();
  const [projectId, setProjectId] = useState<string | null>(lastVisitedProjectId);

  const setAndStoreProjectId = useCallback((id: string | null) => {
    setProjectId(id);
    setLastVisitedProjectId(id);
  }, []);

  const [me, page, project, features, filters] = useQueries({
    queries: [
      users.me(),
      projects.all(),
      { ...projects.get(projectId!), enabled: !!projectId },
      { ...projects.features(projectId!), enabled: !!projectId },
      { ...cases.filters(projectId!), enabled: !!projectId },
    ],
  });

  useEffect(() => {
    // Skip project selection and redirect directly when the user only has access to one project.
    if (page.data?.pagination.totalRecords === 1) {
      setAndStoreProjectId(page.data[0].id);
    }
  }, [page.data]);

  if (
    me.isPending ||
    page.isPending ||
    (projectId && !project.isError && (!project.data || !features.data || !filters.data))
  ) {
    return <LoadingFallback />;
  }

  if (
    me.isError ||
    page.isError ||
    project.isError ||
    features.isError ||
    filters.isError ||
    !me.data ||
    !page.data.pagination.totalRecords
  ) {
    if (projectId) setAndStoreProjectId(null);
    return <AuthorizationFallback />;
  }

  return (
    <MeContext.Provider
      value={{
        id: me.data.id,
        roles: me.data.roles,
        isAdmin: me.data.roles.includes(ADMIN_ROLE_ID),
        timezone: me.data.timezone,
      }}
    >
      <ProjectContext.Provider
        value={{
          projectId,
          projectName: project.data?.name ?? null,
          noveraEnabled: project.data?.agentEnabled ?? false,
          kbReferencesEnabled: project.data?.kbReferencesEnabled ?? false,
          features: features.data,
          projectTypeId: project.data?.typeId,
          type: project.data?.type,

          setProjectId: setAndStoreProjectId,
        }}
      >
        <FiltersContext.Provider value={{ data: filters.data, isLoading: filters.isLoading }}>
          {children}
        </FiltersContext.Provider>
      </ProjectContext.Provider>
    </MeContext.Provider>
  );
}
