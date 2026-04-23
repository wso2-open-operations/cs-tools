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

import { Suspense, useState } from "react";
import { Box, CircularProgress, SearchBar, Stack, Typography, useTheme } from "@wso2/oxygen-ui";
import { Folder } from "@wso2/oxygen-ui-icons-react";
import { ProjectCard } from "@components/features/projects";
import { useNavigate } from "react-router-dom";
import { useProject } from "@context/project";
// import { useSuspenseQuery } from "@tanstack/react-query";
// import { getProjects } from "@src/services/projects";

import { MOCK_PROJECTS } from "@src/mocks/data/projects";

export default function SelectProjectPage() {
  return (
    <Suspense
      fallback={
        <Stack
          bgcolor="components.portal.background.main"
          minHeight="100vh"
          justifyContent="center"
          alignItems="center"
        >
          <CircularProgress color="primary" />
        </Stack>
      }
    >
      <SelectProjectContent />
    </Suspense>
  );
}

function SelectProjectContent() {
  const theme = useTheme();
  const navigate = useNavigate();
  const { setProjectId } = useProject();
  const [search, setSearch] = useState("");

  // const { data } = useSuspenseQuery({
  //   queryKey: ["projects"],
  //   queryFn: getProjects,
  // });

  return (
    <Box minHeight="100vh" px={2.5} py={5}>
      <Stack direction="row" justifyContent="center" alignItems="center" gap={1}>
        <Folder size={24} color={theme.palette.primary.main} />
        <Typography variant="h4" fontWeight="bold">
          Select Your Project
        </Typography>
      </Stack>
      <Typography variant="body2" color="text.secondary" textAlign="center" lineHeight={1.5} px={3} mt={1.5}>
        Choose a project to access your support cases, chat history, and dashboard
      </Typography>
      <Stack mt={5} gap={3}>
        <SearchBar
          size="small"
          placeholder="Search Projects"
          value={search}
          onChange={(event) => setSearch(event.target.value)}
          sx={{ mt: 1 }}
          fullWidth
        />
        {MOCK_PROJECTS.map((props) => (
          <ProjectCard
            key={props.id}
            onClick={() => {
              setProjectId(props.id);
              navigate("/");
            }}
            {...props}
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
