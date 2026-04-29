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

import { type JSX, useEffect } from "react";
import { Box } from "@wso2/oxygen-ui";
import { Outlet, useParams } from "react-router";
import useGetProjectDetails from "@api/useGetProjectDetails";
import useInfiniteProjects, { flattenProjectPages } from "@api/useGetProjects";
import ApiErrorState from "@components/error/ApiErrorState";
import AccountSuspendedPage from "@/components/access-control/AccountSuspendedPage";
import ProjectSuspendedNoticePage from "@/components/access-control/ProjectSuspendedNoticePage";
import { useErrorPageContext } from "@context/error-page/ErrorPageContext";
import { PROJECT_HUB_PROJECTS_PAGE_SIZE } from "@features/project-hub/constants/projectHubConstants";
import { ProjectClosureState } from "@/types/permission";

/**
 * ProjectGuard wraps all routes under `projects/:projectId`.
 *
 * It fetches project details once at the layout boundary. If the API
 * returns an error (400, 401, 403, 404, 5xx, etc.) the guard renders
 * {@link ApiErrorState} instead of the child route. If the project has
 * closureState "Suspended" it renders the Project Suspension Notice.
 *
 * @returns {JSX.Element} The child outlet or an error/suspension page.
 */
function ProjectGuardContent(): JSX.Element {
  const { projectId } = useParams<{ projectId: string }>();
  const { setIsErrorPageDisplayed, setIsProjectSuspended } =
    useErrorPageContext();

  const { data, error, isLoading } = useGetProjectDetails(projectId ?? "");
  const {
    data: projectsData,
    isLoading: isProjectsLoading,
    isFetching: isProjectsFetching,
    hasNextPage,
    fetchNextPage,
  } = useInfiniteProjects({ pageSize: PROJECT_HUB_PROJECTS_PAGE_SIZE });

  const hasError = !isLoading && Boolean(error);
  const isProjectSuspended =
    !isLoading && data?.closureState === ProjectClosureState.SUSPENDED;

  const allProjects = flattenProjectPages(projectsData);
  const allPagesLoaded =
    !isProjectsLoading && !isProjectsFetching && hasNextPage === false;
  const allProjectsSuspended =
    allPagesLoaded &&
    allProjects.length > 0 &&
    allProjects.every((p) => p.closureState === ProjectClosureState.SUSPENDED);

  const isErrorPageDisplayed = hasError || isProjectSuspended;

  useEffect(() => {
    setIsErrorPageDisplayed(isErrorPageDisplayed);
  }, [isErrorPageDisplayed, setIsErrorPageDisplayed]);

  useEffect(() => {
    if (
      isProjectSuspended &&
      hasNextPage &&
      !isProjectsLoading &&
      !isProjectsFetching
    ) {
      fetchNextPage();
    }
  }, [
    isProjectSuspended,
    hasNextPage,
    isProjectsLoading,
    isProjectsFetching,
    fetchNextPage,
  ]);

  useEffect(() => {
    setIsProjectSuspended(isProjectSuspended);
  }, [isProjectSuspended, setIsProjectSuspended]);

  if (hasError) {
    return (
      <Box sx={{ py: 4, px: 2 }}>
        <ApiErrorState
          error={error}
          fallbackMessage="Unable to load project details. Please try again later."
        />
      </Box>
    );
  }

  if (isProjectSuspended) {
    if (!allPagesLoaded) {
      return <></>;
    }
    if (allProjectsSuspended) {
      return <AccountSuspendedPage />;
    }
    return <ProjectSuspendedNoticePage project={data!} />;
  }

  return <Outlet />;
}

export default function ProjectGuard(): JSX.Element {
  return <ProjectGuardContent />;
}
