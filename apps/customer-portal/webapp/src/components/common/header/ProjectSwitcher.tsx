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
  ComplexSelect,
  Header as HeaderUI,
  Skeleton,
} from "@wso2/oxygen-ui";
import { FolderOpen } from "@wso2/oxygen-ui-icons-react";
import type { JSX } from "react";
import { useState } from "react";
import type { ProjectListItem } from "@/types/projects";
import ErrorIndicator from "@components/common/error-indicator/ErrorIndicator";

// Props for the ProjectSwitcher component.
interface ProjectSwitcherProps {
  projects: ProjectListItem[];
  selectedProject?: ProjectListItem;
  onProjectChange: (projectId: string) => void;
  isLoading?: boolean;
  isError?: boolean;
}

const INITIAL_DISPLAY_LIMIT = 10;
const SCROLL_LOAD_THRESHOLD = 200;

/**
 * Project switcher component for the header.
 *
 * @param {ProjectSwitcherProps} props - The props for the component.
 * @returns {JSX.Element} The ProjectSwitcher component.
 */
export default function ProjectSwitcher({
  projects,
  selectedProject,
  onProjectChange,
  isLoading,
  isError,
}: ProjectSwitcherProps): JSX.Element {
  const [displayLimit, setDisplayLimit] = useState(INITIAL_DISPLAY_LIMIT);

  const handleMenuScroll = (event: React.UIEvent<HTMLElement>) => {
    const list = event.currentTarget;
    const scrollTop = list.scrollTop;
    const scrollHeight = list.scrollHeight;
    const clientHeight = list.clientHeight;

    if (
      scrollHeight - (scrollTop + clientHeight) < SCROLL_LOAD_THRESHOLD &&
      displayLimit < projects.length
    ) {
      setDisplayLimit((prev) => Math.min(prev + 10, projects.length));
    }
  };

  if (isLoading) {
    return (
      <HeaderUI.Switchers showDivider={false}>
        <Box
          sx={{
            display: "flex",
            alignItems: "center",
            gap: 1,
            height: 40,
            px: 1.5,
            border: "1px solid",
            borderColor: "action.disabledBackground",
            borderRadius: 0,
          }}
        >
          <FolderOpen size={16} />
          <Skeleton variant="rounded" width={150} height={20} />
        </Box>
      </HeaderUI.Switchers>
    );
  }

  if (isError) {
    return (
      <HeaderUI.Switchers showDivider={false}>
        <Box
          sx={{
            display: "flex",
            alignItems: "center",
            justifyContent: "center",
            gap: 1,
            height: 40,
            width: 200,
            px: 1.5,
            border: "1px solid",
            borderColor: "error.main",
            borderRadius: 0,
            color: "error.main",
          }}
        >
          <ErrorIndicator entityName="Projects" />
        </Box>
      </HeaderUI.Switchers>
    );
  }

  if (projects.length <= 1) {
    const project = selectedProject || projects[0];

    return (
      <HeaderUI.Switchers showDivider={false}>
        <Box
          sx={{
            display: "flex",
            alignItems: "center",
            gap: 1,
            height: 40,
            minWidth: 200,
            px: 1.5,
            border: "1px solid",
            borderColor: "divider",
            borderRadius: 0,
            backgroundColor: "background.paper",
          }}
        >
          <FolderOpen size={16} />
          <ComplexSelect.MenuItem.Text
            primary={project ? project.name : "Select Project"}
          />
        </Box>
      </HeaderUI.Switchers>
    );
  }

  return (
    <HeaderUI.Switchers showDivider={false}>
      {/* project switcher select */}
      <ComplexSelect
        value={selectedProject?.id || ""}
        onChange={(event: any) => onProjectChange(event.target.value)}
        size="small"
        sx={{
          minWidth: 200,
          "& .MuiOutlinedInput-root": {
            "& fieldset": {
              borderColor: "divider",
            },
            "&:hover fieldset": {
              borderColor: "action.active",
            },
            "&.Mui-focused fieldset": {
              borderColor: "primary.main",
            },
          },
        }}
        MenuProps={{
          PaperProps: {
            sx: {
              maxHeight: "320px",
              overflow: "hidden",
              display: "flex",
              flexDirection: "column",
              "& .MuiList-root": {
                flex: 1,
                overflow: "auto",
              },
            },
          },
          MenuListProps: {
            onScroll: handleMenuScroll,
          },
        }}
        renderValue={(selected) => {
          const project = projects.find((project) => project.id === selected);
          return (
            <Box sx={{ display: "flex", alignItems: "center", gap: 1 }}>
              <FolderOpen size={16} />
              <ComplexSelect.MenuItem.Text
                primary={project ? project.name : "Select Project"}
              />
            </Box>
          );
        }}
      >
        <ComplexSelect.ListHeader>Switch Project</ComplexSelect.ListHeader>
        {/* project switcher list items - limited with scroll to load more */}
        {projects.slice(0, displayLimit).map((project) => (
          <ComplexSelect.MenuItem key={project.id} value={project.id}>
            <ComplexSelect.MenuItem.Text
              primary={project.name}
              secondary={project.key}
            />
          </ComplexSelect.MenuItem>
        ))}
        {displayLimit < projects.length && (
          <Box
            sx={{
              py: 1,
              px: 2,
              textAlign: "center",
              fontSize: "0.75rem",
              color: "text.secondary",
              borderTop: "1px solid",
              borderColor: "divider",
            }}
          >
            Scroll to load more ({displayLimit} of {projects.length})
          </Box>
        )}
      </ComplexSelect>
    </HeaderUI.Switchers>
  );
}
