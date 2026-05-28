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
import { Card, pxToRem, Skeleton, Stack } from "@wso2/oxygen-ui";

export function ItemCardSkeleton() {
  return (
    <Card sx={{ p: 1 }}>
      <Stack gap={0.8}>
        <Stack direction="row" justifyContent="space-between" alignItems="center">
          <Stack direction="row" alignItems="center" gap={1}>
            <Skeleton variant="circular" width={pxToRem(18)} height={pxToRem(18)} />
            <Skeleton variant="text" width={60} height={20} />
            <Skeleton variant="rounded" width={50} height={24} sx={{ borderRadius: 1 }} />
          </Stack>
          <Skeleton variant="circular" width={pxToRem(18)} height={pxToRem(18)} />
        </Stack>

        <Skeleton variant="text" width="90%" height={28} />

        <Stack direction="row" alignItems="center" gap={1}>
          <Skeleton variant="rounded" width={70} height={24} sx={{ borderRadius: 1 }} />
          <Skeleton variant="circular" width={4} height={4} />
          <Skeleton variant="text" width={80} height={20} />
        </Stack>

        <Stack gap={0.5} mt={1}>
          <Stack direction="row" alignItems="center" gap={1}>
            <Skeleton variant="circular" width={pxToRem(16)} height={pxToRem(16)} />
            <Skeleton variant="text" width={100} height={18} />
          </Stack>
        </Stack>
      </Stack>
    </Card>
  );
}
