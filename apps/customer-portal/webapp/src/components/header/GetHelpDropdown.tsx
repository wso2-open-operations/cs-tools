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

import {
  Box,
  Button,
  ButtonGroup,
  Divider,
  Menu,
  MenuItem,
  Skeleton,
  Stack,
  Typography,
} from "@wso2/oxygen-ui";
import {
  ChevronDown,
  CircleQuestionMark,
  FileText,
  MessageSquare,
  ShieldAlert,
} from "@wso2/oxygen-ui-icons-react";
import type { JSX } from "react";
import { useMemo, useState } from "react";
import { useNavigate, useParams } from "react-router";
import useInfiniteProjects, { flattenProjectPages } from "@api/useGetProjects";
import useGetProjectFeatures from "@api/useGetProjectFeatures";
import { getProjectPermissions, isProjectRestricted } from "@utils/permission";

interface GetHelpMenuItem {
  id: string;
  label: string;
  description: string;
  icon: React.ReactNode;
  onClick: () => void;
}

/**
 * Get Help dropdown in the header.
 * Shows Issue, Service Request, and Security Report options.
 *
 * @returns {JSX.Element} The Get Help dropdown component.
 */
function GetHelpDropdownContent(): JSX.Element {
  const navigate = useNavigate();
  const { projectId } = useParams<{ projectId?: string }>();

  const {
    data: projectsData,
    isLoading: isProjectsLoading,
    isFetching: isProjectsFetching,
  } = useInfiniteProjects({ pageSize: 20 });
  const projects = useMemo(
    () => flattenProjectPages(projectsData),
    [projectsData],
  );
  const selectedProject = useMemo(
    () => projects.find((p) => p.id === projectId),
    [projects, projectId],
  );
  const { data: projectFeatures, isLoading: isProjectFeaturesLoading } =
    useGetProjectFeatures(projectId || "");
  const areProjectFeaturesReady = !projectId || !!projectFeatures;

  const projectTypeLabel = selectedProject?.type?.label;
  const permissions = areProjectFeaturesReady
    ? getProjectPermissions(projectTypeLabel, {
        projectFeatures,
      })
    : undefined;
  const isServiceRequestVisible = permissions?.hasSR ?? false;
  const isSecurityReportVisible = permissions?.hasSraWriteAccess ?? false;
  const isFeaturesBusy = !!projectId && isProjectFeaturesLoading;
  const isProjectsListBusy =
    isProjectsLoading ||
    (isProjectsFetching && projects.length === 0) ||
    isFeaturesBusy;
  const [anchorEl, setAnchorEl] = useState<null | HTMLElement>(null);
  const open = Boolean(anchorEl);

  const handleClick = (event: React.MouseEvent<HTMLElement>) => {
    setAnchorEl(event.currentTarget);
  };

  const handleClose = () => {
    setAnchorEl(null);
  };

  const handleIssue = () => {
    handleClose();
    if (projectId) {
      const noveraEnabled = selectedProject?.hasAgent ?? false;
      if (noveraEnabled) {
        navigate(`/projects/${projectId}/support/chat/describe-issue`);
      } else {
        navigate(`/projects/${projectId}/support/chat/create-case`, {
          state: { skipChat: true },
        });
      }
    }
  };

  const handleServiceRequest = () => {
    handleClose();
    if (projectId) {
      navigate(`/projects/${projectId}/support/service-requests/create`);
    }
  };

  const handleSecurityReport = () => {
    handleClose();
    if (projectId) {
      navigate(`/projects/${projectId}/support/security-report/create`);
    }
  };

  const menuItems: GetHelpMenuItem[] = [
    {
      id: "issue",
      label: "Issue",
      description: "Report an issue or get help",
      icon: <MessageSquare size={16} />,
      onClick: handleIssue,
    },
    ...(isServiceRequestVisible
      ? [
          {
            id: "service-request",
            label: "Service Request",
            description: "Request deployment assistance",
            icon: <FileText size={16} />,
            onClick: handleServiceRequest,
          },
        ]
      : []),
    ...(isSecurityReportVisible
      ? [
          {
            id: "security-report",
            label: "Security Report",
            description: "Submit a security report",
            icon: <ShieldAlert size={16} />,
            onClick: handleSecurityReport,
          },
        ]
      : []),
  ];

  if (isProjectsListBusy) {
    return (
      <Stack direction="row" alignItems="center" spacing={0.5}>
        <Skeleton variant="rounded" width={96} height={36} />
        <Skeleton variant="rounded" width={36} height={36} />
        <Divider
          orientation="vertical"
          flexItem
          sx={{ mx: 1, display: { xs: "none", sm: "block" } }}
        />
      </Stack>
    );
  }

  return (
    <Box sx={{ display: "flex", alignItems: "center" }}>
      <ButtonGroup variant="contained" color="primary">
        {/* LEFT SIDE (main action) */}
        <Button
          startIcon={<CircleQuestionMark size={16} />}
          onClick={handleIssue}
          sx={{ px: 2 }}
        >
          Get Help
        </Button>

        {/* RIGHT SIDE (dropdown trigger) */}
        <Button
          id="get-help-trigger"
          aria-controls={open ? "get-help-menu" : undefined}
          aria-haspopup="true"
          aria-expanded={open ? "true" : undefined}
          onClick={handleClick}
          sx={{ minWidth: 40, px: 1 }}
        >
          <ChevronDown size={16} />
        </Button>
      </ButtonGroup>

      {/* MENU (unchanged) */}
      <Menu
        id="get-help-menu"
        anchorEl={anchorEl}
        open={open}
        onClose={handleClose}
        MenuListProps={{
          "aria-labelledby": "get-help-trigger",
        }}
        anchorOrigin={{
          vertical: "bottom",
          horizontal: "right",
        }}
        transformOrigin={{
          vertical: "top",
          horizontal: "right",
        }}
        slotProps={{
          paper: {
            sx: { minWidth: 224 },
          },
        }}
      >
        {isProjectsListBusy ? (
          <Box sx={{ px: 2, py: 1.5, width: 224 }}>
            <Skeleton variant="rounded" height={48} sx={{ mb: 1 }} />
            <Skeleton variant="rounded" height={48} sx={{ mb: 1 }} />
            <Skeleton variant="rounded" height={48} />
          </Box>
        ) : (
          menuItems.map((item, index) => (
            <Box key={item.id}>
              {index > 0 && (
                <Divider
                  variant="middle"
                  component="li"
                  sx={{ listStyle: "none" }}
                />
              )}
              <MenuItem onClick={item.onClick}>
                <Box sx={{ display: "flex", alignItems: "flex-start", gap: 1.5 }}>
                  <Box sx={{ display: "flex", alignItems: "center", mt: 0.5 }}>
                    {item.icon}
                  </Box>
                  <Box>
                    <Typography variant="body2" sx={{ fontWeight: 500 }}>
                      {item.label}
                    </Typography>
                    <Typography variant="caption" color="text.secondary">
                      {item.description}
                    </Typography>
                  </Box>
                </Box>
              </MenuItem>
            </Box>
          ))
        )}
      </Menu>

      <Divider
        orientation="vertical"
        flexItem
        sx={{ mx: 1, display: { xs: "none", sm: "block" } }}
      />
    </Box>
  );
}

/**
 * Get Help dropdown wrapper that avoids mounting data hooks
 * when the selected project is in restricted mode.
 *
 * @returns {JSX.Element} Get Help dropdown with optional divider.
 */
export default function GetHelpDropdown(): JSX.Element {
  const { projectId } = useParams<{ projectId?: string }>();
  const {
    data: projectsData,
    isLoading: isProjectsLoading,
    isFetching: isProjectsFetching,
  } = useInfiniteProjects({ pageSize: 20 });
  const projects = useMemo(
    () => flattenProjectPages(projectsData),
    [projectsData],
  );
  const selectedProject = useMemo(
    () => projects.find((p) => p.id === projectId),
    [projects, projectId],
  );
  const isProjectsListBusy =
    isProjectsLoading || (isProjectsFetching && projects.length === 0);
  if (
    !isProjectsListBusy &&
    isProjectRestricted(selectedProject?.closureState)
  ) {
    return <></>;
  }
  return <GetHelpDropdownContent />;
}
