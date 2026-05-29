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
import { Card, Divider, pxToRem, Skeleton, Stack } from "@wso2/oxygen-ui";

export function ItemCardSkeleton() {
  return (
    <Card sx={{ textDecoration: "none", mb: 2 }}>
      <Stack bgcolor="background.paper" p={2} gap={2}>
        <Stack gap={0.8}>
          <Stack direction="row" justifyContent="space-between" gap={5}>
            <Stack direction="row" alignItems="center" flexWrap="wrap" gap={1}>
              <Skeleton variant="circular" width={pxToRem(19)} height={pxToRem(19)} />
              <Skeleton variant="text" width={80} height={20} />
              <Skeleton variant="rounded" width={60} height={24} sx={{ borderRadius: 1 }} />
            </Stack>
            <Stack direction="row" gap={2} alignItems="center">
              <Skeleton variant="rounded" width={70} height={24} sx={{ borderRadius: 1 }} />
              <Skeleton variant="circular" width={pxToRem(18)} height={pxToRem(18)} />
            </Stack>
          </Stack>
          <Stack gap={0.2}>
            <Skeleton variant="text" width="60%" height={28} />
            <Skeleton variant="text" width="85%" height={20} />
            <Skeleton variant="text" width="40%" height={20} />
          </Stack>
        </Stack>
        <Divider />
        <Stack direction="row" justifyContent="space-between" alignItems="center" gap={5}>
          <Stack direction="row" gap={3}>
            <Stack>
              <Skeleton variant="text" width={50} height={16} />
              <Skeleton variant="text" width={90} height={22} />
            </Stack>
          </Stack>
          <Skeleton variant="text" width={120} height={20} />
        </Stack>
      </Stack>
    </Card>
  );
}
