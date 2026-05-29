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
import { useState } from "react";

import {
  alpha,
  Box,
  CircularProgress,
  Popover,
  type PopoverProps,
  pxToRem,
  SearchBar,
  Stack,
  Typography,
} from "@wso2/oxygen-ui";

import { ProjectPopoverList } from "@features/projects/components";

import { ErrorBoundary } from "@components/core";

export function ProjectSelector({ open, anchorEl, onClose }: PopoverProps) {
  const [search, setSearch] = useState("");

  const fallback = (
    <Stack alignItems="center" py={2}>
      <CircularProgress size={20} />
    </Stack>
  );

  return (
    <Popover
      component={Box}
      open={open}
      anchorEl={anchorEl}
      onClose={onClose}
      transformOrigin={{
        vertical: "top",
        horizontal: "center",
      }}
      anchorOrigin={{
        vertical: "top",
        horizontal: "center",
      }}
      slotProps={{
        paper: {
          sx: (theme) => ({
            width: "100%",
            display: "flex",
            flexDirection: "column",
            border: `1px solid ${theme.palette.divider}`,
            boxShadow: `${alpha(theme.palette.text.primary, 0.3)} 0px 48px 100px 0px`,
            maxHeight: 300,
            position: "relative",
          }),
        },
        transition: {
          onExited: () => setSearch(""),
        },
      }}
    >
      <Stack
        sx={{
          gap: 1,
          borderBottom: "1px solid",
          borderColor: "divider",
          bgcolor: "background.paper",
          pt: 1,
          pb: 1.5,
          position: "sticky",
          top: 0,
          left: 0,
          right: 0,
        }}
      >
        <Typography color="text.secondary" sx={{ fontSize: pxToRem(12) }} px={1.5}>
          Select Project
        </Typography>
        <SearchBar
          fullWidth
          placeholder="Search Projects"
          sx={{ px: 1 }}
          onChange={(event) => setSearch(event.target.value)}
        />
      </Stack>
      <ErrorBoundary fallback={fallback}>
        <Box sx={{ flex: 1, overflowY: "auto", py: 1 }}>
          <ProjectPopoverList search={search} onClose={onClose} />
        </Box>
      </ErrorBoundary>
    </Popover>
  );
}
