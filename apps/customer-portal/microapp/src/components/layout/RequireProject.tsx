import { useProject } from "@context/project";
import { Navigate, Outlet } from "react-router-dom";

const RequireProject = () => {
  const { projectId } = useProject();

  if (!projectId) {
    return <Navigate to="/select" replace />;
  }

  return <Outlet />;
};

export default RequireProject;
