// Copyright (c) 2025 WSO2 LLC. (https://www.wso2.com).
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

import React from "react";
import { Box, Button } from "@mui/material";
import type { ProjectSwitcherProps } from "./types";
import { KeyboardArrowDown } from "@mui/icons-material";
import { FolderOpenIcon } from "../../../assets/icons/common-icons";

const ProjectSwitcher: React.FC<ProjectSwitcherProps> = ({
  handleOpen,
  currentProjectName,
  anchorEl,
}) => {
  return (
    <Button
      onClick={handleOpen}
      sx={{
        gap: 1,
        border: (theme) => `1px solid ${theme.palette.grey[300]}`,
        borderRadius: "6px",
        px: 2,
        py: 1,
        textTransform: "none",
        color: "text.primary",
        maxWidth: "280px",
        "&:hover": {
          bgcolor: (theme) => theme.palette.grey[50],
        },
      }}
    >
      {/* Project Switcher Icon */}
      <FolderOpenIcon width="16px" height="16px" color="primary.main" />
      {/* Project Switcher Name */}
      <Box
        sx={{
          overflow: "hidden",
          textOverflow: "ellipsis",
          whiteSpace: "nowrap",
        }}
      >
        {currentProjectName}
      </Box>
      {/* Project Switcher Arrow */}
      <KeyboardArrowDown
        sx={{
          fontSize: "1rem",
          color: "grey.500",
          transition: "transform 0.2s",
          transform: Boolean(anchorEl) ? "rotate(180deg)" : "rotate(0deg)",
        }}
      />
    </Button>
  );
};

export default ProjectSwitcher;
