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
import { Box, SearchBar, Stack, Typography } from "@wso2/oxygen-ui";
import { Folder } from "@wso2/oxygen-ui-icons-react";

import { useNotify } from "@context/snackbar";

import { ProjectsList, ProjectsListSkeleton } from "@features/projects/components";
import { useFilters } from "@features/projects/hooks";

import { ErrorBoundary, ExitButton } from "@shared/components/core";

export default function ProjectSelectPage() {
  const notify = useNotify();
  const { filters, set } = useFilters();

  return (
    <>
      <Stack
        direction="row"
        sx={{
          mb: 4,
          pb: 0.5,
          pt: "var(--safe-top)",
          bgcolor: "background.default",
          position: "sticky",
          top: 0,
          zIndex: 999,
        }}
      >
        <ExitButton />
      </Stack>

      <Stack direction="row" justifyContent="center" alignItems="center" gap={1} px={1.5}>
        <Box color="primary.main">
          <Folder size={24} />
        </Box>
        <Typography variant="h4" fontWeight="bold">
          Select Your Project
        </Typography>
      </Stack>
      <Typography variant="body2" color="text.secondary" textAlign="center" lineHeight={1.5} px={3} mt={1}>
        Choose a project to access your support cases, chat history, and dashboard
      </Typography>

      <Stack sx={{ px: 1.5, gap: 2.5, mt: 3.5 }}>
        <SearchBar
          fullWidth
          size="small"
          placeholder="Search Projects"
          value={filters.search}
          onChange={(e) => set({ search: e.currentTarget.value })}
          sx={{ bgcolor: "background.paper" }}
        />

        <ErrorBoundary
          onError={() => notify.error("Failed to load projects. Try again later.")}
          fallback={<ProjectsListSkeleton />}
        >
          <ProjectsList />
        </ErrorBoundary>
      </Stack>

      <Typography variant="body2" textAlign="center" color="text.secondary" p={5}>
        Need access to another project? Contact your administrator
      </Typography>
    </>
  );
}
