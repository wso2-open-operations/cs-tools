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

import { Box, Card, Chip, Typography } from "@wso2/oxygen-ui";
import type { JSX } from "react";

interface CsmComingSoonPageProps {
  title: string;
  description: string;
  blockedOn?: string;
}

/**
 * Placeholder for CSM top-level sections whose cross-customer backend
 * endpoints have not yet been delivered by `csm-portal/backend`. Used
 * deliberately over a 404 so the sidebar feels real.
 */
export default function CsmComingSoonPage({
  title,
  description,
  blockedOn,
}: CsmComingSoonPageProps): JSX.Element {
  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 3 }}>
      <Box sx={{ display: "flex", alignItems: "center", gap: 1.5 }}>
        <Typography variant="h5">{title}</Typography>
        <Chip size="small" label="Coming soon" color="warning" variant="outlined" />
      </Box>
      <Card sx={{ p: 3, display: "flex", flexDirection: "column", gap: 1.5 }}>
        <Typography variant="body1">{description}</Typography>
        {blockedOn && (
          <Typography variant="body2" color="text.secondary">
            Blocked on: {blockedOn}
          </Typography>
        )}
      </Card>
    </Box>
  );
}
