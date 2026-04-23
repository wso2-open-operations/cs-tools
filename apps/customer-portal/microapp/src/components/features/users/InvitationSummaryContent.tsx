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

import { Chip, Stack, Typography } from "@wso2/oxygen-ui";
import type { RoleName } from "./RoleSelector";

interface InvitationSummaryProps {
  projectName?: string;
  email: string;
  name: string;
  role: RoleName;
}

export function InvitationSummaryContent({ projectName, email, name, role }: InvitationSummaryProps) {
  const summary = [
    { label: "Project", value: projectName || "-" },
    { label: "User Email", value: email || "-" },
    { label: "User Name", value: name || "-" },
    { label: "Role", value: <Chip size="small" label={role} color={role === "Admin" ? "primary" : "default"} /> },
    { label: "Delivery Method", value: "Email" },
  ];

  return (
    <Stack gap={1}>
      {summary.map((item) => (
        <Stack direction="row" justifyContent="space-between">
          <Typography variant="body2" color="text.secondary">
            {item.label}
          </Typography>
          <Typography component="span" variant="body2">
            {item.value}
          </Typography>
        </Stack>
      ))}
    </Stack>
  );
}
