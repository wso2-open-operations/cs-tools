import { useState } from "react";
import { ProjectContext } from "./ProjectContext";

export default function ProjectProvider({ children }: { children: React.ReactNode }) {
  const [projectId, setProjectId] = useState<string | null>(null);

  return <ProjectContext.Provider value={{ projectId, setProjectId }}>{children}</ProjectContext.Provider>;
}
