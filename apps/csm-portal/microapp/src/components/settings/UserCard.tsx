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

import { Card, Chip, Skeleton, Stack, Typography } from "@wso2/oxygen-ui";
import type { AdminUser } from "@src/types";
import { INTERNAL_USER_ROLES } from "./config";

// Mobile-card equivalent of the webapp's CsmUsersPage table row: username, name, email, role
// chips (or the postgres-source userType when roles aren't present), active/inactive status, and
// timezone — same fields, same fallbacks ("—").
export function UserCard({ user }: { user: AdminUser }) {
  return (
    <Card sx={{ p: 1.5 }}>
      <Stack gap={0.75}>
        <Stack direction="row" justifyContent="space-between" alignItems="center">
          <Typography variant="subtitle2" color="text.secondary">
            {user.userName}
          </Typography>
          {user.active !== undefined && (
            <Chip
              size="small"
              label={user.active ? "Active" : "Inactive"}
              color={user.active ? "success" : "default"}
              variant="outlined"
            />
          )}
        </Stack>

        <Typography variant="body1" color="text.primary" noWrap>
          {user.name || "—"}
        </Typography>
        <Typography variant="body2" color="text.secondary" noWrap>
          {user.email}
        </Typography>

        <Stack direction="row" alignItems="center" gap={1} flexWrap="wrap">
          {user.roles && user.roles.length > 0 ? (
            user.roles.map((role) => (
              <Chip
                key={role}
                size="small"
                label={role}
                color={(INTERNAL_USER_ROLES as string[]).includes(role) ? "primary" : "default"}
                variant="outlined"
              />
            ))
          ) : user.userType ? (
            <Chip
              size="small"
              label={user.userType}
              color={user.userType === "internal" ? "primary" : "default"}
              variant="outlined"
            />
          ) : null}
        </Stack>

        <Typography variant="subtitle2" color="text.secondary" sx={{ fontSize: 12 }}>
          {user.timezone ?? "—"}
        </Typography>
      </Stack>
    </Card>
  );
}

export function UserCardSkeleton() {
  return (
    <Card sx={{ p: 1.5 }}>
      <Stack gap={0.75}>
        <Stack direction="row" justifyContent="space-between" alignItems="center">
          <Skeleton variant="text" width={80} height={20} />
          <Skeleton variant="rounded" width={60} height={22} sx={{ borderRadius: 1 }} />
        </Stack>
        <Skeleton variant="text" width="60%" height={26} />
        <Skeleton variant="text" width="80%" height={20} />
        <Stack direction="row" gap={1}>
          <Skeleton variant="rounded" width={70} height={24} sx={{ borderRadius: 1 }} />
        </Stack>
        <Skeleton variant="text" width={100} height={18} />
      </Stack>
    </Card>
  );
}
