import React, { createContext, useContext, type ReactNode } from "react";
import { useParams } from "react-router-dom";
import { keepPreviousData } from "@tanstack/react-query";
import { useGet } from "../services/useApi";
import { Endpoints } from "../services/endpoints";
import { PROJECTS_LIST_CACHE_KEY } from "../utils/constants";
import type { Project, ProjectResponse } from "../types/project.types";

interface ProjectContextType {
  projects: Project[];
  currentProject: Project | undefined;
  isLoading: boolean;
  isError: boolean;
  totalRecords: number;
}

const ProjectContext = createContext<ProjectContextType | undefined>(undefined);

export const ProjectProvider: React.FC<{ children: ReactNode }> = ({
  children,
}) => {
  const { sysId } = useParams<{ sysId: string }>();

  // Fetch all projects without pagination
  const { data, isLoading, isError } = useGet<ProjectResponse>(
    [PROJECTS_LIST_CACHE_KEY],
    Endpoints.getAllProjects(0, 999999),
    {
      placeholderData: keepPreviousData,
    }
  );

  const projects = data?.projects || [];
  const totalRecords = data?.pagination?.totalRecords || 0;

  // Find the current project from URL
  const currentProject = projects.find(
    (project) => project.sysId.toString() === sysId
  );

  return (
    <ProjectContext.Provider
      value={{
        projects,
        currentProject,
        isLoading,
        isError,
        totalRecords,
      }}
    >
      {children}
    </ProjectContext.Provider>
  );
};

export const useProject = (): ProjectContextType => {
  const context = useContext(ProjectContext);
  if (context === undefined) {
    throw new Error("useProject must be used within a ProjectProvider");
  }
  return context;
};
