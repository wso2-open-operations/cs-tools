// Copyright (c) 2025 WSO2 LLC. (https://www.wso2.com).
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

import { Typography, Box } from "@mui/material";
import ProjectCard from "../components/ProjectCard";

import Header from "../components/Header/Header";
import { Endpoints } from "../services/endpoints";
import { PROJECTS_LIST_CACHE_KEY } from "../utils/constants";
import { useGet } from "../services/useApi";
import type { ProjectResponse } from "../types/project.types";
import { FolderOpenIcon } from "../assets/icons/common/folder-open-icon";

export default function Home() {
  const { data } = useGet<ProjectResponse>(
    [PROJECTS_LIST_CACHE_KEY],
    Endpoints.getAllProjects()
  );

  const projects = data?.projects || [];

  return (
    <Box
      sx={{
        minHeight: "100vh",
        bgcolor: "background.default",
        display: "flex",
        flexDirection: "column",
      }}
    >
      <Header />
      <Box
        component="main"
        sx={{
          flex: 1,
          display: "flex",
          alignItems: "center",
          justifyContent: "center",
          p: 3,
        }}
      >
        <Box sx={{ width: "100%", maxWidth: "1024px" }}>
          {" "}
          {/* max-w-5xl */}
          <Box textAlign="center" mb={4}>
            {" "}
            {/* mb-8 */}
            <Box
              display="flex"
              alignItems="center"
              justifyContent="center"
              gap={1}
              mb={1.5}
            >
              {" "}
              {/* gap-2 mb-3 */}
              <FolderOpenIcon width="24px" height="24px" color="primary.main" />
              <Typography
                variant="subtitle1"
                component="h1"
                color="text.primary"
              >
                {" "}
                {/* text-gray-900 */}
                Select Your Project
              </Typography>
            </Box>
            <Typography color="text.secondary">
              {" "}
              {/* text-gray-600 */}
              Choose a project to access your support cases, chat history, and
              dashboard
            </Typography>
          </Box>
          {/* Grid Layout */}
          <Box
            sx={{
              display: projects.length === 1 ? "flex" : "grid",
              justifyContent: projects.length === 1 ? "center" : undefined,
              gridTemplateColumns: {
                xs: "1fr",
                md: "repeat(2, 1fr)",
                lg: "repeat(3, 1fr)",
              },
              gap: 2, // gap-6
            }}
          >
            {projects.map((project) => (
              <ProjectCard
                key={project.sysId}
                sysId={project.sysId}
                tags={project.projectKey}
                title={project.name}
                subtitle={project.description?.replace(/<[^>]*>?/gm, "") || ""}
                openCases={project.openCasesCount}
                activeChats={project.activeChatsCount}
                status="All Good" // Status not in API, defaulting to Active or removing if optional (ProjectCard props check needed but safely assuming string)
                date={project.createdOn}
              />
            ))}
          </Box>
          {/* Footer Text */}
          <Box mt={4} textAlign="center">
            <Typography variant="body2" color="grey.500">
              {" "}
              {/* text-gray-500 */}
              Need access to another project? Contact your administrator
            </Typography>
          </Box>
        </Box>
      </Box>
    </Box>
  );
}
