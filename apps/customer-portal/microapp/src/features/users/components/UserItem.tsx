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
import { Box, Card, pxToRem, Skeleton, Stack, Typography, useTheme } from "@wso2/oxygen-ui";
import { ChevronRight, Crown, Mail } from "@wso2/oxygen-ui-icons-react";

import { UserAvatar } from "@features/users/components";
import type { Role, User } from "@features/users/types";

import { ROLES } from "@shared/constants";
import { useNavigation } from "@shared/hooks";

export interface UserItemProps {
  name: string;
  email: string;
  role: Role;
  lastActive: string;
}

export function UserItem(props: User) {
  const theme = useTheme();
  const { toEditUser } = useNavigation();

  return (
    <Card sx={{ textDecoration: "none", p: 1 }} onClick={() => toEditUser(props)}>
      <Stack direction="row" alignItems="center" justifyContent="space-between" gap={2}>
        <Stack direction="row" alignItems="center" gap={2}>
          <UserAvatar>{props.firstName}</UserAvatar>

          <Stack minWidth={0}>
            <Stack direction="row" gap={1} alignItems="center">
              <Typography
                variant="subtitle1"
                fontWeight="medium"
                sx={{
                  display: "-webkit-box",
                  WebkitLineClamp: 1,
                  WebkitBoxOrient: "vertical",
                  wordBreak: "break-word",
                  textOverflow: "ellipsis",
                  overflow: "hidden",
                }}
              >
                {`${props.firstName} ${props.lastName}`}
              </Typography>

              {props.roles.includes(ROLES.ADMIN) && (
                <Box sx={{ color: "primary.main" }}>
                  <Crown size={18} />
                </Box>
              )}
            </Stack>

            <Stack direction="row" alignItems="center" gap={1}>
              <Mail color={theme.palette.text.secondary} size={pxToRem(13)} />
              <Typography variant="subtitle2" fontWeight="regular" color="text.secondary">
                {props.email}
              </Typography>
            </Stack>
          </Stack>
        </Stack>

        <ChevronRight color={theme.palette.text.secondary} size={18} style={{ flexShrink: 0 }} />
      </Stack>
    </Card>
  );
}

export function UserItemSkeleton() {
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
