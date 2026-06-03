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
import { Box, LinearProgress } from "@wso2/oxygen-ui";
import { Outlet, useParams } from "react-router";
import useGetProjectDetails from "@api/useGetProjectDetails";
import ApiErrorState from "@components/error/ApiErrorState";
import SuspendedProjectBanner from "@features/project-hub/components/SuspendedProjectBanner";
import { useErrorPageContext } from "@context/error-page/ErrorPageContext";
import { ProjectClosureState } from "@/types/permission";

/**
 * ProjectGuard wraps all routes under `projects/:projectId`.
 *
 * It fetches project details once at the layout boundary. If the API
 * returns an error (400, 401, 403, 404, 5xx, etc.) the guard renders
 * {@link ApiErrorState} instead of the child route. Suspended projects are
 * still accessible — internal teams need to investigate suspended accounts
 * — so we render the normal outlet with a non-blocking banner on top.
 *
 * @returns {JSX.Element} The child outlet or an error page.
 */
function ProjectGuardContent(): JSX.Element {
  const { projectId } = useParams<{ projectId: string }>();
  const { setIsErrorPageDisplayed } = useErrorPageContext();

  const { data, error, isLoading } = useGetProjectDetails(projectId ?? "");

  const hasError = !isLoading && Boolean(error);
  const isProjectSuspended =
    !isLoading && data?.closureState === ProjectClosureState.SUSPENDED;

  useEffect(() => {
    setIsErrorPageDisplayed(hasError);
    return () => setIsErrorPageDisplayed(false);
  }, [hasError, setIsErrorPageDisplayed]);

  if (isLoading) {
    return (
      <Box
        sx={{
          flex: 1,
          display: "flex",
          justifyContent: "center",
          alignItems: "center",
        }}
      >
        <LinearProgress sx={{ width: "60%", maxWidth: 400 }} />
      </Box>
    );
  }

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

  return (
    <>
      {isProjectSuspended && data && <SuspendedProjectBanner project={data} />}
      <Outlet />
    </>
  );
}

export default function ProjectGuard(): JSX.Element {
  return <ProjectGuardContent />;
}
