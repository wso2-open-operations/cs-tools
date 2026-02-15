import { useContext } from "react";
import { ProjectContext } from "./ProjectContext";

export function useProject() {
  const context = useContext(ProjectContext);

  if (!context) {
    throw new Error("useProject must be used within an ProjectProvider");
  }

  return context;
}
