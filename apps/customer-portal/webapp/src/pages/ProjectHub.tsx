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

import { Box, Button, Typography } from "@wso2/oxygen-ui";
import { useNavigate } from "react-router";
import { type JSX } from "react";
import useSearchProjects from "@/api/useSearchProjects";
import { useLogger } from "@/hooks/useLogger";

/**
 * ProjectHub component.
 *
 * @returns {JSX.Element} The ProjectHub component.
 */
export default function ProjectHub(): JSX.Element {
  /**
   * Navigation hook.
   */
  const navigate = useNavigate();

  /**
   * Logger hook.
   */
  const logger = useLogger();

  /**
   * Fetch projects from the API.
   */
  const {
    data: projectsResponse,
    isLoading,
    isError,
  } = useSearchProjects({}, true);

  /**
   * Get projects from the response.
   */
  const projects =
    projectsResponse?.pages.flatMap((page) => page.projects) || [];

  /**
   * Render content based on the state.
   */
  const renderContent = () => {
    if (isLoading) {
      return (
        <Typography variant="h6" color="text.secondary">
          Loading projects...
        </Typography>
      );
    }

    /**
     * Log error if projects are not loaded.
     */
    if (isError) {
      logger.error("Failed to load projects in ProjectHub");
      return (
        <Typography variant="h6" color="error">
          Error loading projects. Please try again later.
        </Typography>
      );
    }

    /**
     * Log error if projects are not loaded.
     */
    if (!projects || projects.length === 0) {
      return (
        <Typography variant="h6" color="text.secondary">
          No projects available.
        </Typography>
      );
    }

    /**
     * Log projects if projects are loaded.
     */
    if (projects.length > 0) {
      logger.debug(`${projects.length} projects loaded in ProjectHub`);
    }

    return (
      <Box
        sx={{
          display: "grid",
          gap: 4,
          maxWidth: "1200px",
          mx: "auto",
        }}
      >
        {/* project card */}
        {projects.map((project) => (
          <Button
            key={project.id}
            variant="contained"
            fullWidth
            id={project.id}
            onClick={() => navigate(`/${project.key}/dashboard`)}
            sx={{ py: 4 }}
          >
            {project.name}
          </Button>
        ))}
      </Box>
    );
  };

  return (
    <Box sx={{ p: 8, textAlign: "center" }}>
      {/* project hub title */}
      <Typography variant="h3" gutterBottom>
        Welcome to Project Hub
      </Typography>
      {/* project hub subtitle */}
      <Typography variant="h6" color="text.secondary" sx={{ mb: 6 }}>
        Select a project to get started
      </Typography>
      {renderContent()}
    </Box>
  );
}
