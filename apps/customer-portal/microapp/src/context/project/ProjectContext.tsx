import { createContext } from "react";

export type ProjectContextType = {
  projectId: string | null;

  setProjectId: (id: string | null) => void;
};

export const ProjectContext = createContext<ProjectContextType | null>(null);
