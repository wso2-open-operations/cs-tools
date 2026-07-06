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

import { Box, Chip, Paper, Typography } from "@wso2/oxygen-ui";
import { type JSX } from "react";

interface IssuesListUnavailableProps {
  title: string;
  description: string;
}

/**
 * Placeholder for an issue kind that has no backend search endpoint yet
 * (change requests, incidents). Keeps the tab structure consistent with the
 * live, case-typed lists while making clear the listing isn't wired.
 */
export default function IssuesListUnavailable({
  title,
  description,
}: IssuesListUnavailableProps): JSX.Element {
  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 1.5 }}>
      <Box sx={{ display: "flex", alignItems: "center", gap: 1 }}>
        <Typography variant="body2" color="text.secondary">
          {description}
        </Typography>
        <Chip size="small" label="Coming soon" color="warning" variant="outlined" />
      </Box>
      <Paper
        variant="outlined"
        sx={{
          p: 5,
          display: "flex",
          flexDirection: "column",
          alignItems: "center",
          gap: 0.5,
          textAlign: "center",
        }}
      >
        <Typography variant="subtitle2">{title} aren&apos;t available yet</Typography>
        <Typography variant="body2" color="text.secondary">
          The list will appear here once the backend search endpoint is available.
        </Typography>
      </Paper>
    </Box>
  );
}
