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

import { Box, CircularProgress, Stack, Typography } from "@mui/material";
import { FolderOpen } from "@mui/icons-material";
import { ProjectCard } from "@features/projects";
import { useSuspenseQuery } from "@tanstack/react-query";
import { getProjects } from "@src/services/projects";
import { Suspense } from "react";

export default function HomePage() {
  return (
    <Suspense
      fallback={
        <Stack bgcolor="semantic.portal.background.main" minHeight="100vh" justifyContent="center" alignItems="center">
          <CircularProgress color="primary" />
        </Stack>
      }
    >
      <HomeContent />
    </Suspense>
  );
}

function HomeContent() {
  const { data } = useSuspenseQuery({
    queryKey: ["projects"],
    queryFn: getProjects,
  });

  return (
    <Box bgcolor="semantic.portal.background.main" minHeight="100vh" px={2.5} py={5}>
      <Stack direction="row" justifyContent="center" alignItems="center" gap={1}>
        <FolderOpen color="primary" fontSize="large" />
        <Typography variant="h4" fontWeight="bold">
          Select Your Project
        </Typography>
      </Stack>
      <Typography variant="body2" color="text.secondary" textAlign="center" lineHeight={1.5} px={3} mt={1.5}>
        Choose a project to access your support cases, chat history, and dashboard
      </Typography>
      <Stack mt={5} gap={3}>
        {data.map((project) => (
          <ProjectCard
            key={project.id}
            id={project.id}
            name={project.name}
            description={project.description}
            type={project.type}
            status={project.status}
            metrics={project.metrics}
          />
        ))}
      </Stack>
      <Box p={5}>
        <Typography variant="body2" textAlign="center" color="text.secondary">
          Need access to another project? Contact your administrator
        </Typography>
      </Box>
    </Box>
  );
}
