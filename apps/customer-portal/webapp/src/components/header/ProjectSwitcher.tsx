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

import { Box, ComplexSelect, Header as HeaderUI } from "@wso2/oxygen-ui";
import { FolderOpen } from "@wso2/oxygen-ui-icons-react";
import type { JSX } from "react";
import type { ProjectListItem } from "@/models/responses";

/**
 * Props for the ProjectSwitcher component.
 */
interface ProjectSwitcherProps {
  /**
   * List of projects.
   */
  projects: ProjectListItem[];
  /**
   * Currently selected project.
   */
  selectedProject?: ProjectListItem;
  /**
   * Callback function to handle project change.
   */
  onProjectChange: (projectKey: string) => void;
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
}: ProjectSwitcherProps): JSX.Element {
  return (
    <HeaderUI.Switchers showDivider={false}>
      {/* project switcher select */}
      <ComplexSelect
        value={selectedProject?.key || ""}
        onChange={(event: any) => onProjectChange(event.target.value)}
        size="small"
        sx={{ minWidth: 200 }}
        renderValue={(selected) => {
          /**
           * Find the project from the URL parameters.
           */
          const project = projects.find((project) => project.key === selected);
          /**
           * Return the project name and icon.
           */
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
          <ComplexSelect.MenuItem key={project.key} value={project.key}>
            <ComplexSelect.MenuItem.Text
              primary={project.name}
              secondary={project.description}
            />
          </ComplexSelect.MenuItem>
        ))}
      </ComplexSelect>
    </HeaderUI.Switchers>
  );
}
