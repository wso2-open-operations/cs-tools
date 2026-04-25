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

import dayjs from "dayjs";
import relativeTime from "dayjs/plugin/relativeTime";
import { Link } from "react-router-dom";
import { Card, Stack, Avatar as MuiAvatar, Typography, Chip, useTheme, pxToRem, Skeleton } from "@wso2/oxygen-ui";
import { ChevronRight, Mail } from "@wso2/oxygen-ui-icons-react";
import { capitalize, stringAvatar } from "@utils/others";
import type { Role, User } from "@src/types";

dayjs.extend(relativeTime);

export interface UserListItemProps {
  name: string;
  email: string;
  role: Role;
  lastActive: string;
}

export function UserListItem({ firstName, lastName, email, roles }: User) {
  const theme = useTheme();

  return (
    <Card
      component={Link}
      elevation={0}
      to="/users/edit"
      state={{ email, firstName, lastName, role: roles[0] }}
      sx={{ textDecoration: "none", p: 1 }}
    >
      <Stack direction="row" alignItems="center" justifyContent="space-between" gap={2}>
        <Stack direction="row" alignItems="center" gap={2}>
          <Avatar>{firstName}</Avatar>

          <Stack minWidth={0}>
            <Stack direction="row" gap={1} alignItems="center">
              <Typography
                variant="subtitle1"
                fontWeight="medium"
                sx={{
                  overflow: "hidden",
                  textOverflow: "ellipsis",
                  display: "-webkit-box",
                  WebkitLineClamp: 1,
                  WebkitBoxOrient: "vertical",
                  wordBreak: "break-word",
                }}
              >
                {`${firstName} ${lastName}`}
              </Typography>
              {roles.length > 0 && roles[0] !== "Portal User" && <Chip size="small" label={capitalize(roles[0])} />}
            </Stack>

            <Stack direction="row" alignItems="center" gap={1}>
              <Mail color={theme.palette.text.secondary} size={pxToRem(13)} />
              <Typography variant="subtitle2" fontWeight="regular" color="text.secondary">
                {email}
              </Typography>
            </Stack>
          </Stack>
        </Stack>

        <ChevronRight color={theme.palette.text.secondary} size={pxToRem(18)} style={{ flexShrink: 0 }} />
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

export function UserListItemSkeleton() {
  return (
    <Card elevation={0} sx={{ p: 1, pointerEvents: "none" }}>
      <Stack direction="row" alignItems="center" gap={2}>
        <Skeleton variant="circular" width={40} height={40} />

        <Stack>
          <Stack direction="row" gap={1} alignItems="center">
            <Skeleton variant="text" width={120} height={24} />
            <Skeleton variant="rounded" width={50} height={20} />
          </Stack>

          <Stack direction="row" alignItems="center" gap={1}>
            <Skeleton variant="circular" width={13} height={13} />
            <Skeleton variant="text" width={180} height={20} />
          </Stack>
        </Stack>
      </Stack>
    </Card>
  );
}
