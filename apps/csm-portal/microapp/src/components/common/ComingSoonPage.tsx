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

import { Card, Chip, Stack, Typography } from "@wso2/oxygen-ui";

interface ComingSoonPageProps {
  title: string;
  description: string;
  blockedOn?: string;
}

// Placeholder for microapp sections not yet built, mirroring the webapp's CsmComingSoonPage
// (apps/csm-portal/webapp/src/features/csm-coming-soon/pages/CsmComingSoonPage.tsx) — used
// deliberately so a reserved tab/route feels real instead of dead or missing.
export function ComingSoonPage({ title, description, blockedOn }: ComingSoonPageProps) {
  return (
    <Stack gap={3}>
      <Stack direction="row" alignItems="center" gap={1.5}>
        <Typography variant="h5">{title}</Typography>
        <Chip size="small" label="Coming soon" color="warning" variant="outlined" />
      </Stack>
      <Card sx={{ p: 3, display: "flex", flexDirection: "column", gap: 1.5 }}>
        <Typography variant="body1">{description}</Typography>
        {blockedOn && (
          <Typography variant="body2" color="text.secondary">
            Blocked on: {blockedOn}
          </Typography>
        )}
      </Card>
    </Stack>
  );
}
