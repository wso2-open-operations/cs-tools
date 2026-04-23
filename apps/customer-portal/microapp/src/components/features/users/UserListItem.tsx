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

import { Link } from "react-router-dom";
import { Card, Stack, Avatar as MuiAvatar, Typography, Chip, useTheme, pxToRem } from "@wso2/oxygen-ui";
import { ShieldUser, ChevronRight, Mail } from "@wso2/oxygen-ui-icons-react";
import { capitalize, stringAvatar } from "@utils/others";
import type { RoleName } from "./RoleSelector";

export interface UserListItemProps {
  name: string;
  email: string;
  role: RoleName;
  lastActive: string;
}

export function UserListItem({ name, email, role, lastActive }: UserListItemProps) {
  const theme = useTheme();
  const admin = role === "admin";

  return (
    <Card
      component={Link}
      elevation={0}
      to="/users/edit"
      state={{ name, email, role: capitalize(role) }}
      sx={{ textDecoration: "none", p: 1 }}
    >
      <Stack direction="row" alignItems="center" justifyContent="space-between" gap={2}>
        <Stack direction="row" alignItems="center" gap={2}>
          <Avatar>{name}</Avatar>
          <Stack>
            <Stack direction="row" gap={1} alignItems="center">
              <Typography variant="subtitle1" fontWeight="medium">
                {name}
              </Typography>
              <Chip size="small" label={capitalize(role)} />
            </Stack>
            <Stack direction="row" alignItems="center" gap={1}>
              <Mail color={theme.palette.text.secondary} size={pxToRem(13)} />
              <Typography variant="subtitle2" fontWeight="regular" color="text.secondary">
                {email}
              </Typography>
            </Stack>
            <Typography variant="caption" fontWeight="regular" color="text.secondary" mt={0.5}>
              Last Active: {lastActive}
            </Typography>
          </Stack>
        </Stack>
        <Stack direction="row" alignItems="center" gap={2}>
          {admin && <ShieldUser color={theme.palette.primary.main} size={pxToRem(28)} />}

          <ChevronRight color={theme.palette.text.secondary} size={pxToRem(18)} />
        </Stack>
      </Stack>
    </Card>
  );
}

export function Avatar({ children }: { children: string }) {
  return (
    <MuiAvatar
      sx={(theme) => ({
        height: 40,
        width: 40,
        bgcolor: "primary.main",
        fontSize: theme.typography.h5,
        fontWeight: "medium",
      })}
    >
      {stringAvatar(children)}
    </MuiAvatar>
  );
}
