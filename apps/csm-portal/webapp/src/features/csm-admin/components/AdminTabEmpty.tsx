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

import { Box, Card, Typography } from "@wso2/oxygen-ui";
import type { JSX } from "react";

interface AdminTabEmptyProps {
  /** What's missing, e.g. "response templates", "SLA policies". */
  resource: string;
}

/**
 * Placeholder shown on admin tabs whose backend endpoints aren't wired yet.
 * Rendered when the mock-data toggle is off — explains why the tab is empty
 * rather than leaking the seeded mock content.
 */
export default function AdminTabEmpty({
  resource,
}: AdminTabEmptyProps): JSX.Element {
  return (
    <Card variant="outlined" sx={{ p: 4 }}>
      <Box
        sx={{
          display: "flex",
          flexDirection: "column",
          alignItems: "center",
          gap: 0.5,
          textAlign: "center",
        }}
      >
        <Typography variant="subtitle2">No {resource}</Typography>
        <Typography variant="body2" color="text.secondary">
          Backend endpoints for {resource} are not yet wired. Enable the mock
          data toggle to see seeded examples.
        </Typography>
      </Box>
    </Card>
  );
}
