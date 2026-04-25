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

import { useLayoutEffect, useRef, useState } from "react";
import { useNavigate } from "react-router-dom";
import {
  AppBar as MuiAppBar,
  Button,
  Chip,
  IconButton,
  Stack,
  Typography,
  pxToRem,
  useTheme,
  alpha,
  Skeleton,
} from "@wso2/oxygen-ui";

import { ProjectSelector } from "@components/features/projects";
import { useLayout } from "@src/context/layout";
import { useProject } from "@context/project";

import { APP_BAR_CONFIG } from "@components/layout/config";
import { PROJECT_STATUS_META } from "@config/constants";
import { ArrowLeft, ChevronDown, Folder, Grip } from "@wso2/oxygen-ui-icons-react";
import { useSuspenseQuery } from "@tanstack/react-query";
import { projects } from "@src/services/projects";
import { goToMyAppsScreen } from "../microapp-bridge";
import { useThemeMode } from "@root/src/context/theme";
import { ConfirmDialog } from "../shared/ConfirmDialog";

export function AppBar() {
  const theme = useTheme();
  const navigate = useNavigate();
  const { title, appBarVariant, overlineSlot, subtitleSlot, startSlot, endSlot, appBarSlots, hasBackAction } =
    useLayout();
  const config = APP_BAR_CONFIG[appBarVariant];
  const { projectId } = useProject();
  const { data: projectsData } = useSuspenseQuery(projects.all());
  const project = projectsData?.find((project) => project.id === projectId);
  const hasMultipleProjects = projectsData.length > 1;

  const [projectSelectorAnchor, setProjectSelectorAnchor] = useState<HTMLButtonElement | null>(null);
  const isProjectSelectorOpen = Boolean(projectSelectorAnchor);

  const ref = useRef<HTMLDivElement>(null);

  useLayoutEffect(() => {
    if (!ref.current) return;

    const observer = new ResizeObserver(([entry]) => {
      document.documentElement.style.setProperty("--app-bar-height", `${entry.contentRect.height}px`);
    });

    observer.observe(ref.current);
    return () => observer.disconnect();
  }, []);

  const navigateBack = () => navigate(-1);

  const openProjectSelector = (event: React.MouseEvent<HTMLButtonElement>) => {
    if (!hasMultipleProjects) return;
    event.preventDefault();
    setProjectSelectorAnchor(event.currentTarget);
  };

  const closeProjectSelector = () => {
    setProjectSelectorAnchor(null);
  };

  const mode = useThemeMode();

  if (!project) return null;

  const statusChipColorVariant = project.status ? PROJECT_STATUS_META[project.status].color : "default";

  return (
    <>
      <MuiAppBar
        ref={ref}
        position="sticky"
        elevation={0}
        sx={{
          backgroundColor: `${mode === "light" ? "white" : "black"} !important`,
          display: "flex",
          flexDirection: "column",
          gap: 1,
          p: 1.5,
          pt: 7,
        }}
      >
        {config.showNotifications && (
          <Stack direction="row" justifyContent="space-between" alignItems="center" mb={0.5}>
            <ExitButton />
          </Stack>
        )}

        <Stack direction="row" justifyContent="space-between" alignItems="center" gap={1}>
          <Stack direction="row" alignItems="center" gap={1.5} flex={1}>
            {hasBackAction && <BackButton onClick={navigateBack} />}
            {startSlot}
            <Stack flex={1}>
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
          <Button sx={{ justifyContent: "space-between", p: 0, mt: 2 }} onClick={openProjectSelector} disableRipple>
            <Stack direction="row" sx={{ alignItems: "center", flexGrow: 1, minWidth: 0, gap: 1 }}>
              <Folder color={theme.palette.text.secondary} size={pxToRem(18)} />
              <Typography variant="body1" color="text.secondary" sx={{ textTransform: "initial" }} noWrap>
                {project.name}
              </Typography>
            </Stack>
            {hasMultipleProjects && <ChevronDown color={theme.palette.text.secondary} size={pxToRem(18)} />}
          </Button>
        )}

        {config.showChips && (
          <Stack direction="row" gap={2} mt={1.5}>
            {project.status ? (
              <Chip
                label={project.status}
                size="small"
                sx={(theme) => ({
                  bgcolor: alpha(theme.palette[statusChipColorVariant].light, 0.1),
                  color: theme.palette[statusChipColorVariant].light,
                })}
              />
            ) : (
              <Skeleton variant="rounded" width={80} height={24} />
            )}
            <Chip label={project.type} size="small" sx={{ alignSelf: "start" }} />
          </Stack>
        )}

        {/* Additional AppBar Content */}
        {appBarSlots}
      </MuiAppBar>

      {/* Popovers */}
      {hasMultipleProjects && (
        <ProjectSelector anchorEl={projectSelectorAnchor} open={isProjectSelectorOpen} onClose={closeProjectSelector} />
      )}
    </>
  );
}

function BackButton({ onClick }: { onClick: () => void }) {
  return (
    <IconButton aria-label="Go back" onClick={onClick} sx={{ p: 0 }} disableRipple>
      <ArrowLeft size={pxToRem(20)} />
    </IconButton>
  );
}

export function ExitButton() {
  const [open, setOpen] = useState(false);

  return (
    <>
      <IconButton
        disableRipple
        color="error"
        sx={{
          gap: 1,
          position: "absolute",
          top: "var(--safe-top)",
          left: 10,
          p: 0,
        }}
        onClick={() => setOpen(true)}
      >
        <Grip size={pxToRem(20)} />
        <Typography>Go to Apps</Typography>
      </IconButton>

      <ConfirmDialog
        open={open}
        title="Return to Apps"
        description="Are you sure you want to leave this application?"
        confirmColor="error"
        confirmLabel="Leave"
        onClose={() => setOpen(false)}
        onConfirm={goToMyAppsScreen}
      />
    </>
  );
}
