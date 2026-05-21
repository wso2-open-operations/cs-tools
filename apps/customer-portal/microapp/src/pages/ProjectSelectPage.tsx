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
    <Box sx={{ minHeight: "100dvh", px: 2.5, py: 5, mt: "var(--safe-top)" }}>
      <Stack direction="row" justifyContent="space-between" alignItems="center" mb={0.5}>
        <ExitButton />
      </Stack>

      <Stack direction="row" justifyContent="center" alignItems="center" gap={1}>
        <Box color="primary.main">
          <Folder size={24} />
        </Box>
        <Typography variant="h4" fontWeight="bold">
          Select Your Project
        </Typography>
      </Stack>
      <Typography variant="body2" color="text.secondary" textAlign="center" lineHeight={1.5} px={3} mt={1.5}>
        Choose a project to access your support cases, chat history, and dashboard
      </Typography>

      <SearchBar
        fullWidth
        size="small"
        placeholder="Search Projects"
        value={filters.search}
        onChange={(e) => set({ search: e.currentTarget.value })}
        sx={{ mt: 4, bgcolor: "background.paper" }}
      />

      <ErrorBoundary
        onError={() => notify.error("Failed to load projects. Try again later.")}
        fallback={<ProjectsListSkeleton />}
      >
        <ProjectsList />
      </ErrorBoundary>

      <Typography variant="body2" textAlign="center" color="text.secondary" p={5}>
        Need access to another project? Contact your administrator
      </Typography>
    </Box>
  );
}
