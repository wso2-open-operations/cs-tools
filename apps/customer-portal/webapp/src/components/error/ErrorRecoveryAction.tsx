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
import { type JSX, useMemo } from "react";
import { Button } from "@wso2/oxygen-ui";
import { useLocation, useNavigate, useParams } from "react-router";
import useInfiniteProjects, {
  flattenProjectPages,
  getTotalRecords,
} from "@api/useGetProjects";
import { setLastSelectedProject } from "@features/settings/utils/settingsStorage";

/**
 * Renders one primary button that leads a user off an access/not-found error
 * page to somewhere they can actually use, based on the projects the user can
 * access (the same first-page query the header uses, so it shares its cache):
 *
 * - URL project is in the user's list and the failure is on a sub-page:
 *   "Back to project" (that project's dashboard).
 * - Otherwise, exactly one project: "Go to <project name>".
 * - Otherwise: "Go to your projects" (project hub).
 *
 * Renders nothing while the project list is loading or unavailable to the
 * caller (e.g. not signed in), so the button never flashes a wrong target.
 * Navigation only: access is still decided by the backend.
 *
 * @returns {JSX.Element | null} The recovery button, or null until resolved.
 */
export default function ErrorRecoveryAction(): JSX.Element | null {
  const navigate = useNavigate();
  const { pathname } = useLocation();
  const { projectId } = useParams<{ projectId?: string }>();

  const { data, isLoading, isError, isFetched } = useInfiniteProjects({
    pageSize: 20,
  });
  const projects = useMemo(() => flattenProjectPages(data), [data]);
  const totalRecords = getTotalRecords(data);

  const target = useMemo(() => {
    const accessibleProject = projectId
      ? projects.find((p) => p.id === projectId)
      : undefined;
    const dashboardPath = accessibleProject
      ? `/projects/${accessibleProject.id}/dashboard`
      : undefined;

    // On the dashboard itself "back to project" would loop to the same page.
    if (accessibleProject && dashboardPath && pathname !== dashboardPath) {
      return {
        label: "Back to project",
        path: dashboardPath,
        projectId: accessibleProject.id,
      };
    }
    if (totalRecords === 1 && projects.length === 1) {
      return {
        label: `Go to ${projects[0].name}`,
        path: `/projects/${projects[0].id}/dashboard`,
        projectId: projects[0].id,
      };
    }
    return { label: "Go to your projects", path: "/", projectId: undefined };
  }, [projectId, projects, totalRecords, pathname]);

  if (isLoading || (!isFetched && !isError)) {
    return null;
  }

  return (
    <Button
      variant="contained"
      onClick={() => {
        if (target.projectId) {
          setLastSelectedProject({ id: target.projectId });
        }
        navigate(target.path);
      }}
    >
      {target.label}
    </Button>
  );
}
