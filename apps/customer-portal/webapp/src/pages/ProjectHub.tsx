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

import { Box, Typography } from "@wso2/oxygen-ui";
import { useEffect, useMemo, type JSX } from "react";
import { useNavigate } from "react-router";
import useGetProjects from "@api/useGetProjects";
import { useLogger } from "@hooks/useLogger";
import { useLoader } from "@context/linear-loader/LoaderContext";
import ProjectCard from "@components/project-hub/project-card/ProjectCard";
import ProjectCardSkeleton from "@components/project-hub/project-card/ProjectCardSkeleton";
import { FolderOpen } from "@wso2/oxygen-ui-icons-react";
import { useAsgardeo } from "@asgardeo/react";
import EmptyIcon from "@components/common/empty-state/EmptyIcon";
import ErrorStateIcon from "@components/common/error-state/ErrorStateIcon";

/**
 * ProjectHub component.
 *
 * @returns {JSX.Element} The ProjectHub component.
 */
export default function ProjectHub(): JSX.Element {
  const logger = useLogger();
  const navigate = useNavigate();
  const { showLoader, hideLoader } = useLoader();
  const { isLoading: isAuthLoading } = useAsgardeo();
  const {
    data: projectsResponse,
    isLoading,
    isError,
  } = useGetProjects({}, true);

  const projects = useMemo(
    () => projectsResponse?.projects || [],
    [projectsResponse?.projects],
  );

  useEffect(() => {
    if (isLoading || isAuthLoading) {
      showLoader();
      return () => hideLoader();
    }
  }, [isLoading, isAuthLoading, showLoader, hideLoader]);

  // Navigate to dashboard if there is only one project
  useEffect(() => {
    if (!isLoading && !isAuthLoading && !isError && projects.length === 1) {
      navigate(`/${projects[0].id}/dashboard`, { replace: true });
    }
  }, [projects, isLoading, isAuthLoading, isError, navigate]);

  useEffect(() => {
    if (isError) {
      logger.error("Failed to load projects in ProjectHub");
    }
  }, [isError, logger]);

  useEffect(() => {
    if (projects.length > 0) {
      logger.debug(`${projects.length} projects loaded in ProjectHub`);
    }
  }, [projects.length, logger]);

  const renderContent = () => {
    if (isLoading || isAuthLoading) {
      return (
        <Box
          sx={{
            display: "flex",
            flexWrap: "wrap",
            justifyContent: "center",
            gap: 3,
            maxWidth: 1800,
            mx: "auto",
            width: "100%",
          }}
        >
          {[...Array(3)].map((_, index) => (
            <Box
              key={index}
              sx={{
                flex: {
                  xs: "1 1 100%",
                  sm: "0 1 calc(50% - 24px)",
                  md: "0 1 calc(33.33% - 24px)",
                  lg: "0 1 calc(25% - 24px)",
                  xl: "0 1 calc(20% - 24px)",
                },
                maxWidth: {
                  xs: "100%",
                  sm: 400,
                },
                minWidth: 300,
              }}
            >
              <ProjectCardSkeleton />
            </Box>
          ))}
        </Box>
      );
    }

    if (isError) {
      return (
        <Box
          sx={{
            display: "flex",
            flexDirection: "column",
            alignItems: "center",
            justifyContent: "center",
            gap: 2,
            py: 10,
          }}
        >
          <ErrorStateIcon />
          <Typography variant="h4">Something Went Wrong</Typography>
          <Typography variant="subtitle2" color="text.secondary">
            We couldn&apos;t load the data right now. Please try again or refresh
            the page.
          </Typography>
        </Box>
      );
    }

    if (projects.length === 0) {
      return (
        <Box
          sx={{
            display: "flex",
            flexDirection: "column",
            alignItems: "center",
            justifyContent: "center",
            gap: 2,
            py: 10,
          }}
        >
          <EmptyIcon />
          <Typography variant="h4">No Projects Yet</Typography>
          <Typography variant="subtitle2" color="text.secondary">
            Projects will appear here once they are created or assigned to you
          </Typography>
        </Box>
      );
    }

    return (
      <Box
        sx={{
          display: "flex",
          flexWrap: "wrap",
          justifyContent: "center",
          gap: 3,
          maxWidth: 1800,
          mx: "auto",
          width: "100%",
        }}
      >
        {/* project card wrapper */}
        {projects.map((project) => (
          <Box
            key={project.id}
            sx={{
              flex: {
                xs: "1 1 100%",
                sm: "0 1 calc(50% - 24px)",
                md: "0 1 calc(33.33% - 24px)",
                lg: "0 1 calc(25% - 24px)",
                xl: "0 1 calc(20% - 24px)",
              },
              maxWidth: {
                xs: "100%",
                sm: 400,
              },
              minWidth: 300,
            }}
          >
            <ProjectCard
              id={project.id}
              projectKey={project.key}
              title={project.name}
              subtitle={project.description}
              date={project.createdOn}
            />
          </Box>
        ))}
      </Box>
    );
  };

  return (
    <Box
      sx={{
        width: "100%",
        minHeight: "100%",
        display: "flex",
        flexDirection: "column",
      }}
    >
      <Box
        sx={{
          width: "100%",
          display: "flex",
          flexDirection: "column",
          flex: 1,
          justifyContent: "center",
        }}
      >
        {!(isError || (!isLoading && !isAuthLoading && projects.length === 0)) && (
          <Box
            sx={{
              mb: 3,
              display: "flex",
              flexDirection: "column",
              alignItems: "center",
              textAlign: "center",
            }}
          >
            {/* project hub title */}
            <Box
              sx={{
                mb: 1,
                display: "flex",
                alignItems: "center",
                gap: 1.5,
              }}
            >
              <FolderOpen size={28} />
              <Typography variant="h4">Select Your Project</Typography>
            </Box>

            {/* project hub subtitle */}
            <Typography variant="subtitle2" color="text.secondary">
              Choose a project to access your support cases, chat history, and
              dashboard
            </Typography>
          </Box>
        )}
        <Box sx={{ width: "100%" }}>{renderContent()}</Box>
      </Box>
    </Box>
  );
}
