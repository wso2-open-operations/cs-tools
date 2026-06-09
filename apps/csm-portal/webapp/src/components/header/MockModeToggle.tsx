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

import { Box, Switch, Tooltip, Typography } from "@wso2/oxygen-ui";
import type { JSX } from "react";
import { useMockMode } from "@context/mock-mode/MockModeContext";

/**
 * Header switch to flip between seeded mock data and the live backend at
 * runtime. Rendered only when the deployment opts in via
 * `window.config.CSM_PORTAL_ALLOW_MOCK_TOGGLE`, so it never appears in a
 * production build where an engineer could unknowingly act on fake data.
 *
 * The flag flip is handled by `MockModeProvider`, which persists the choice to
 * localStorage and invalidates every cached query so the next render refetches
 * through the newly-selected code path.
 */
export default function MockModeToggle(): JSX.Element | null {
  // Hook must run unconditionally; the visibility gate comes after it.
  const { useMocks, toggle } = useMockMode();

  if (!window.config?.CSM_PORTAL_ALLOW_MOCK_TOGGLE) {
    return null;
  }

  return (
    <Tooltip
      title={
        useMocks
          ? "Showing seeded mock data — switch to the live backend"
          : "Using the live backend — switch to mock data"
      }
    >
      <Box
        component="label"
        sx={{
          display: "flex",
          alignItems: "center",
          gap: 0.5,
          cursor: "pointer",
          userSelect: "none",
        }}
      >
        <Typography
          variant="caption"
          sx={{
            color: useMocks ? "warning.main" : "text.secondary",
            fontWeight: 600,
          }}
        >
          Mock
        </Typography>
        <Switch
          size="small"
          color="warning"
          checked={useMocks}
          onChange={toggle}
          inputProps={{ "aria-label": "Toggle mock data" }}
        />
      </Box>
    </Tooltip>
  );
}
