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

import { Box, Typography } from "@mui/material";
import React from "react";
import { SupportIcon } from "../../../assets/icons/side-nav-bar-icons";

interface SupportHeaderProps {
  projectName?: string;
}

export const SupportHeader: React.FC<SupportHeaderProps> = ({
  projectName,
}) => {
  return (
    <Box
      sx={{
        backgroundColor: "background.paper",
        borderBottom: 1,
        borderColor: "grey.200",
        px: 4,
        py: 3,
      }}
    >
      <Box sx={{ display: "flex", alignItems: "center", gap: 1.5, mb: 1 }}>
        <Box
          sx={{
            p: 1,
            backgroundColor: "#ffedd5", // orange-100
            borderRadius: "8px",
            color: "#ea580c", // orange-600
            display: "flex",
            alignItems: "center",
            justifyContent: "center",
          }}
        >
          <SupportIcon width={24} height={24} />
        </Box>
        <Box>
          <Typography
            variant="subtitle1"
            color="text.primary"
            sx={{ fontWeight: 400 }}
          >
            Support
          </Typography>
          <Typography variant="body2" color="text.secondary">
            Support center for {projectName}
          </Typography>
        </Box>
      </Box>
    </Box>
  );
};
