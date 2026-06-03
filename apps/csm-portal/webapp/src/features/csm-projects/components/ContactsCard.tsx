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

import {
  Avatar,
  Box,
  Card,
  Chip,
  Link,
  Typography,
} from "@wso2/oxygen-ui";
import type { JSX } from "react";
import { initialsOf } from "@utils/userClaims";
import type {
  CsmContact,
  CsmContactRole,
  CsmContactStatus,
} from "@features/csm-projects/types/csmProjects";

interface ContactsCardProps {
  contacts: CsmContact[];
  /** Card title; defaults to "Contacts". */
  title?: string;
  /** Optional caption shown beneath the title. */
  subtitle?: string;
  /** When true, marks each contact with its inheritance scope chip. */
  showScope?: boolean;
}

function statusColor(
  status: CsmContactStatus,
): "success" | "warning" | "default" {
  if (status === "Active") return "success";
  if (status === "Invited") return "warning";
  return "default";
}

function roleColor(
  role: CsmContactRole,
): "primary" | "info" | "warning" | "default" {
  switch (role) {
    case "Account Manager":
      return "primary";
    case "Technical Owner":
      return "info";
    case "Security Contact":
      return "warning";
    default:
      return "default";
  }
}

/**
 * Card showing the people associated with an account or project. Each row
 * lists name + email + status chip + one chip per role. `showScope` adds a
 * small chip marking project-specific contacts when an account view inlines
 * inherited entries.
 */
export default function ContactsCard({
  contacts,
  title = "Contacts",
  subtitle,
  showScope = false,
}: ContactsCardProps): JSX.Element {
  return (
    <Card sx={{ p: 2.5, display: "flex", flexDirection: "column", gap: 1.5 }}>
      <Box sx={{ display: "flex", alignItems: "baseline", gap: 1 }}>
        <Typography variant="subtitle2">{title}</Typography>
        <Typography variant="caption" color="text.secondary">
          {contacts.length} entr{contacts.length === 1 ? "y" : "ies"}
        </Typography>
      </Box>
      {subtitle && (
        <Typography variant="caption" color="text.secondary">
          {subtitle}
        </Typography>
      )}
      {contacts.length === 0 ? (
        <Typography variant="body2" color="text.secondary">
          No contacts on file.
        </Typography>
      ) : (
        <Box
          sx={{
            display: "grid",
            gridTemplateColumns: {
              xs: "1fr",
              sm: "repeat(2, minmax(0, 1fr))",
              lg: "repeat(3, minmax(0, 1fr))",
            },
            gap: 1.25,
          }}
        >
          {contacts.map((c) => (
            <Box
              key={c.id}
              sx={{
                display: "flex",
                alignItems: "flex-start",
                gap: 1.25,
                p: 1.25,
                border: 1,
                borderColor: "divider",
                borderRadius: 1,
                minWidth: 0,
              }}
            >
              <Avatar sx={{ width: 32, height: 32, fontSize: 13 }}>
                {initialsOf(c.name)}
              </Avatar>
              <Box sx={{ flex: 1, minWidth: 0, display: "flex", flexDirection: "column", gap: 0.5 }}>
                <Box
                  sx={{
                    display: "flex",
                    alignItems: "center",
                    gap: 0.75,
                    flexWrap: "wrap",
                  }}
                >
                  <Typography variant="body2" fontWeight={500} noWrap>
                    {c.name}
                  </Typography>
                  <Chip
                    size="small"
                    variant="outlined"
                    color={statusColor(c.status)}
                    label={c.status}
                    sx={{ height: 18, fontSize: 10 }}
                  />
                  {showScope && c.scope === "project" && (
                    <Chip
                      size="small"
                      variant="outlined"
                      label="Project"
                      sx={{ height: 18, fontSize: 10 }}
                    />
                  )}
                </Box>
                <Link
                  href={`mailto:${c.email}`}
                  underline="hover"
                  sx={{ fontSize: 12, wordBreak: "break-all" }}
                >
                  {c.email}
                </Link>
                {c.phone && (
                  <Typography variant="caption" color="text.secondary">
                    {c.phone}
                  </Typography>
                )}
                <Box
                  sx={{
                    display: "flex",
                    gap: 0.5,
                    flexWrap: "wrap",
                    mt: 0.25,
                  }}
                >
                  {c.roles.map((r) => (
                    <Chip
                      key={r}
                      size="small"
                      color={roleColor(r)}
                      variant="outlined"
                      label={r}
                      sx={{ height: 20, fontSize: 11 }}
                    />
                  ))}
                </Box>
              </Box>
            </Box>
          ))}
        </Box>
      )}
    </Card>
  );
}
