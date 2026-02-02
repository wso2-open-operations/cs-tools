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
import type { ProjectListItem } from "@/models/responses";

// Props for the ProjectSwitcher component.
interface ProjectSwitcherProps {
  projects: ProjectListItem[];
  selectedProject?: ProjectListItem;
  onProjectChange: (projectId: string) => void;
  isLoading?: boolean;
}

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
}: ProjectSwitcherProps): JSX.Element {
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
            borderRadius: "4px",
          }}
        >
          <FolderOpen size={16} />
          <Skeleton variant="rounded" width={150} height={20} />
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
        sx={{ minWidth: 200 }}
        renderValue={(selected) => {
          /**
           * Find the project from the URL parameters.
           */
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
        {/* project switcher list items */}
        {projects.map((project) => (
          <ComplexSelect.MenuItem key={project.id} value={project.id}>
            <ComplexSelect.MenuItem.Text
              primary={project.name}
              secondary={project.key}
            />
          </ComplexSelect.MenuItem>
        ))}
      </ComplexSelect>
    </HeaderUI.Switchers>
  );
}
