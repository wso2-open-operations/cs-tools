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
import { Box } from "@mui/material";
import { useLocation, useNavigate } from "react-router-dom";
import Header from "../Header/Header";
import Sidebar from "../Sidebar/Sidebar";
import { useProject } from "../../context/ProjectContext";

interface DashboardLayoutProps {
  children: React.ReactNode;
}

const DashboardLayout: React.FC<DashboardLayoutProps> = ({ children }) => {
  const location = useLocation();
  const navigate = useNavigate();

  const { currentProject } = useProject();

  // Handle project switching
  const handleProjectChange = (newProjectId: string) => {
    // Get the current page path (e.g., "/dashboard", "/support", etc.)
    const pathSegments = location.pathname.split("/");
    const currentPage = pathSegments[pathSegments.length - 1]; // Last segment is the page

    // Navigate to the same page type but with the new project ID
    navigate(`/${newProjectId}/${currentPage}`);
  };

  return (
    <Box
      sx={{
        display: "grid",
        gridTemplateColumns: "auto 1fr",
        height: "100vh",
        overflow: "hidden",
        bgcolor: "background.default",
      }}
    >
      {/* Fixed Sidebar */}
      <Box sx={{ gridRow: "1 / -1" }}>
        <Sidebar
          projectName={currentProject?.name}
          projectKey={currentProject?.projectKey}
          projectId={currentProject?.sysId}
        />
      </Box>

      {/* Main area with Header and Content */}
      <Box
        sx={{
          display: "flex",
          flexDirection: "column",
          height: "100vh",
          overflow: "hidden",
        }}
      >
        <Header
          variant="dashboard"
          projectName={currentProject?.name}
          projectKey={currentProject?.projectKey}
          currentProjectId={currentProject?.sysId}
          onProjectChange={handleProjectChange}
        />

        {/* Scrollable content area */}
        <Box
          component="main"
          sx={{
            flex: 1,
            overflowY: "auto",
            bgcolor: "grey.50",
          }}
        >
          {children}
        </Box>
      </Box>
    </Box>
  );
};

export default DashboardLayout;
