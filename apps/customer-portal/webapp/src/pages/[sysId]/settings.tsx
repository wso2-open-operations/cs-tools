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
import { SettingsIcon } from "@/assets/icons/side-nav-bar-icons";

export default function Settings() {
  return (
    <>
      <Box
        sx={{
          bgcolor: "white",
          borderBottom: 1,
          borderColor: "grey.200",
          px: 4,
          py: 3,
        }}
      >
        <Box display="flex" alignItems="center" gap={1.5} mb={1}>
          <Box
            sx={{
              display: "flex",
              alignItems: "center",
              justifyContent: "center",
              p: 1,
              bgcolor: "#e5e7eb",
              borderRadius: "8px",
              color: "#374151",
            }}
          >
            <SettingsIcon width="24px" height="24px" color="currentColor" />
          </Box>
          <Box>
            <Typography
              variant="h6"
              component="h1"
              sx={{
                color: "grey.900",
                fontWeight: 600,
                lineHeight: 1.2,
                fontSize: "1.25rem",
              }}
            >
              Settings
            </Typography>
            <Typography variant="body2" color="grey.500" fontSize="0.875rem">
              Project settings and configuration
            </Typography>
          </Box>
        </Box>
      </Box>
      <Box sx={{ px: 4, py: 3 }}>
        <Typography variant="body1" color="grey.600">
          Settings content coming soon...
        </Typography>
      </Box>
    </>
  );
}
