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

import React, { Suspense, useEffect, useState } from "react";
import { Box, SearchBar, Stack, Typography, useTheme } from "@wso2/oxygen-ui";
import { Folder } from "@wso2/oxygen-ui-icons-react";
import { ProjectCard, ProjectCardSkeleton } from "@components/features/projects";
import { useNavigate } from "react-router-dom";
import { useProject } from "@context/project";
import { useInfiniteQuery } from "@tanstack/react-query";
import { projects } from "@src/services/projects";
import { ErrorBoundary, ExitButton } from "@components/core";
import EmptyState from "../components/shared/EmptyState";
import { useNotify } from "../context/snackbar";
import { InfiniteScroll } from "../components/shared";
import type { GetAllProjectsRequestDto } from "../types";

export default function SelectProjectPage() {
  const theme = useTheme();
  const notify = useNotify();
  const [search, setSearch] = useState("");

  return (
    <Box minHeight="100vh" px={2.5} py={5} mt="var(--safe-top)">
      <Stack direction="row" justifyContent="space-between" alignItems="center" mb={0.5}>
        <ExitButton />
      </Stack>
      <Stack direction="row" justifyContent="center" alignItems="center" gap={1}>
        <Folder size={24} color={theme.palette.primary.main} />
        <Typography variant="h4" fontWeight="bold">
          Select Your Project
        </Typography>
      </Stack>
      <Typography variant="body2" color="text.secondary" textAlign="center" lineHeight={1.5} px={3} mt={1.5}>
        Choose a project to access your support cases, chat history, and dashboard
      </Typography>
      <Stack mt={3} gap={3}>
        <SearchBar
          size="small"
          placeholder="Search Projects"
          value={search}
          onChange={(e) => setSearch(e.currentTarget.value)}
          sx={{ mt: 1, bgcolor: "background.paper" }}
          fullWidth
        />
        <ErrorBoundary
          onError={() => notify.error("Failed to load projects. Try again later.")}
          fallback={<ProjectsListContentSkeleton />}
        >
          <Suspense fallback={<ProjectsListContentSkeleton />}>
            <ProjectsListContent search={search} />
          </Suspense>
        </ErrorBoundary>
      </Stack>
      <Box p={5}>
        <Typography variant="body2" textAlign="center" color="text.secondary">
          Need access to another project? Contact your administrator
        </Typography>
      </Box>
    </Box>
  );
}

function ProjectsListContent({ search }: { search: string }) {
  const navigate = useNavigate();
  const { setProjectId } = useProject();
  const filters: GetAllProjectsRequestDto["filters"] = search
    ? {
        searchQuery: search,
      }
    : {};

  const query = useInfiniteQuery(projects.paginated({ filters }));

  const totalRecords = query.data?.pages[0].pagination.totalRecords;

  useEffect(() => {
    if (totalRecords !== 1) return;
    if (search) return;

    const project = query.data?.pages[0][0];
    if (project) {
      setProjectId(project.id);
      navigate("/", { replace: true });
    }
  }, [query.data]);

  return (
    <InfiniteScroll {...query} sentinel={<ProjectsListContentSkeleton />} tail={totalRecords === 0 && <EmptyState />}>
      {(data) => (
        <>
          {data &&
            data.pages.map((page, pageIndex) => (
              <React.Fragment key={pageIndex}>
                {page.map((item) => (
                  <ProjectCard
                    key={item.id}
                    {...item}
                    onClick={() => {
                      setProjectId(item.id);
                      navigate("/");
                    }}
                  />
                ))}
              </React.Fragment>
            ))}
        </>
      )}
    </InfiniteScroll>
  );
}

function ProjectsListContentSkeleton() {
  return (
    <>
      {Array.from({ length: 5 }).map((_, index) => (
        <ProjectCardSkeleton key={index} />
      ))}
    </>
  );
}
