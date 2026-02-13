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

import { Card, CardContent, Stack, Typography } from "@wso2/oxygen-ui";
import { type ReactNode, type JSX } from "react";

export interface CaseDetailsCardProps {
  title: string;
  icon: ReactNode;
  children: ReactNode;
}

/**
 * Reusable card for case details sections with a consistent title row (icon + heading).
 *
 * @param {CaseDetailsCardProps} props - Title, icon, and children.
 * @returns {JSX.Element} The card with title and content.
 */
export default function CaseDetailsCard({
  title,
  icon,
  children,
}: CaseDetailsCardProps): JSX.Element {
  return (
    <Card variant="outlined">
      <CardContent sx={{ p: 3, "&:last-child": { pb: 3 } }}>
        <Stack spacing={3}>
          <Stack direction="row" alignItems="center" spacing={1}>
            {icon}
            <Typography variant="h6" color="text.primary">
              {title}
            </Typography>
          </Stack>
          {children}
        </Stack>
      </CardContent>
    </Card>
  );
}
