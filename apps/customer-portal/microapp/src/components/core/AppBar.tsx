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
import { Link, useNavigate } from "react-router-dom";
import {
  AppBar as MuiAppBar,
  Button,
  Chip,
  IconButton,
  Stack,
  Typography,
  pxToRem,
  useTheme,
  Box,
  alpha,
} from "@wso2/oxygen-ui";

import { NotificationBadge } from "@components/ui";
import { ProjectSelector } from "@components/features/projects";
import { useLayout } from "@src/context/layout";
import { useProject } from "@context/project";

import { APP_BAR_CONFIG } from "@components/layout/config";
import { MOCK_PROJECTS } from "@src/mocks/data/projects";
import { PROJECT_STATUS_META } from "@config/constants";
import { ArrowLeft, Bell, ChevronDown, Folder } from "@wso2/oxygen-ui-icons-react";

export function AppBar() {
  const theme = useTheme();
  const navigate = useNavigate();
  const { title, appBarVariant, overlineSlot, subtitleSlot, startSlot, endSlot, appBarSlots, hasBackAction } =
    useLayout();
  const config = APP_BAR_CONFIG[appBarVariant];
  const { projectId } = useProject();
  const project = MOCK_PROJECTS.find((project) => project.id === projectId);

  const [projectSelectorAnchor, setProjectSelectorAnchor] = useState<HTMLButtonElement | null>(null);
  const isProjectSelectorOpen = Boolean(projectSelectorAnchor);

  const navigateBack = () => navigate(-1);

  const openProjectSelector = (event: React.MouseEvent<HTMLButtonElement>) => {
    event.preventDefault();
    setProjectSelectorAnchor(event.currentTarget);
  };

  const closeProjectSelector = () => {
    setProjectSelectorAnchor(null);
  };

  if (!project) return null;

  const statusChipColorVariant = PROJECT_STATUS_META[project.status].color;

  return (
    <>
      <MuiAppBar
        position="relative"
        color="transparent"
        elevation={0}
        sx={{ backgroundColor: "background.paper", display: "flex", flexDirection: "column", gap: 1, p: 1.5, pt: 3 }}
      >
        {config.showNotifications && (
          <Box mb={1}>
            <NotificationButton to="/notifications" />
          </Box>
        )}

        <Stack direction="row" justifyContent="space-between" alignItems="center" gap={1}>
          <Stack direction="row" alignItems="center" gap={1.5}>
            {hasBackAction && <BackButton onClick={navigateBack} />}
            {startSlot}
            <Stack>
              <Typography component="div" variant="body2" fontWeight="regular" color="text.secondary">
                {overlineSlot}
              </Typography>
              {title && (
                <Typography variant="h6" fontWeight="medium">
                  {title}
                </Typography>
              )}
              <Typography component="div" variant="subtitle2" fontWeight="regular" color="text.secondary">
                {subtitleSlot}
              </Typography>
            </Stack>
          </Stack>

          {endSlot}
        </Stack>

        {config.showProjectSelector && (
          <Button sx={{ justifyContent: "space-between", p: 0 }} onClick={openProjectSelector} disableRipple>
            <Stack direction="row" sx={{ alignItems: "center" }} gap={1}>
              <Folder color={theme.palette.text.secondary} size={pxToRem(18)} />
              <Typography variant="body1" color="text.secondary" sx={{ textTransform: "initial" }}>
                {project.name}
              </Typography>
            </Stack>
            <ChevronDown color={theme.palette.text.secondary} size={pxToRem(18)} />
          </Button>
        )}
        {config.showChips && (
          <Stack direction="row" gap={2} mt={1.5}>
            <Chip
              label={project.status}
              size="small"
              sx={(theme) => ({
                bgcolor: alpha(theme.palette[statusChipColorVariant].light, 0.1),
                color: theme.palette[statusChipColorVariant].light,
              })}
            />
            <Chip label={project.type} size="small" sx={{ alignSelf: "start" }} />
          </Stack>
        )}

        {/* Additional AppBar Content */}
        {appBarSlots}
      </MuiAppBar>

      {/* Popovers */}
      <ProjectSelector anchorEl={projectSelectorAnchor} open={isProjectSelectorOpen} onClose={closeProjectSelector} />
    </>
  );
}

function NotificationButton({ to }: { to: string }) {
  const theme = useTheme();

  return (
    <IconButton
      aria-label="Open notifications"
      component={Link}
      to={to}
      sx={{
        position: "absolute",
        right: 10,
        top: 10,
        p: 0,
      }}
    >
      <Bell size={pxToRem(20)} color={theme.palette.text.secondary} />
      <NotificationBadge badgeContent={1} color="primary" overlap="circular" />
    </IconButton>
  );
}

function BackButton({ onClick }: { onClick: () => void }) {
  return (
    <IconButton aria-label="Go back" onClick={onClick} sx={{ p: 0 }} disableRipple>
      <ArrowLeft size={pxToRem(20)} />
    </IconButton>
  );
}
