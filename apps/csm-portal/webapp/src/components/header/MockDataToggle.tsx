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

import { Box, Chip, Switch, Tooltip, Typography } from "@wso2/oxygen-ui";
import type { JSX } from "react";
import { useMockMode } from "@context/mock-mode/MockModeContext";

/**
 * Header switch for flipping between seeded mock data and the real backend.
 *
 * On toggle the query cache is invalidated (see MockModeProvider) so the
 * next render fetches through whichever code path the new flag selects.
 * Persists to localStorage and rehydrates on page reload.
 */
export default function MockDataToggle(): JSX.Element {
  const { useMocks, toggle } = useMockMode();
  const label = useMocks ? "Mock data" : "Live backend";
  return (
    <Tooltip
      title={
        useMocks
          ? "Currently serving mocked, in-memory data. Click to call the real backend."
          : "Currently calling the configured backend. Click to switch to mocked data."
      }
      placement="bottom"
    >
      <Box
        sx={{
          display: "flex",
          alignItems: "center",
          gap: 0.75,
          px: 1,
          py: 0.25,
          borderRadius: 1,
          border: 1,
          borderColor: useMocks ? "warning.main" : "divider",
          backgroundColor: useMocks ? "warning.50" : "transparent",
        }}
      >
        <Chip
          size="small"
          variant={useMocks ? "filled" : "outlined"}
          color={useMocks ? "warning" : "default"}
          label={useMocks ? "MOCK" : "LIVE"}
          sx={{ height: 20, fontSize: 10, fontWeight: 600 }}
        />
        <Typography
          variant="caption"
          color={useMocks ? "warning.dark" : "text.secondary"}
          sx={{ display: { xs: "none", sm: "inline" } }}
        >
          {label}
        </Typography>
        <Switch
          size="small"
          checked={useMocks}
          onChange={toggle}
          color="warning"
          inputProps={{ "aria-label": "Toggle mock data" }}
        />
      </Box>
    </Tooltip>
  );
}
