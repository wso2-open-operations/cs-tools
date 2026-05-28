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

export function BubbleAgentSkeleton() {
  return (
    <Stack direction="row" justifyContent="start" width="100%">
      <Card
        component={Stack}
        sx={{
          p: 1.5,
          width: "100%",
          bgcolor: "background.paper",
          borderStyle: "dashed",
          borderWidth: 1,
          borderColor: "divider",
        }}
      >
        <Stack direction="row" justifyContent="start" gap={1} mb={1.5}>
          <Skeleton variant="circular" width={pxToRem(18)} height={pxToRem(18)} />
          <Skeleton variant="text" width={60} height={20} />
        </Stack>

        <Stack gap={1}>
          <Skeleton variant="text" width="90%" height={20} />
          <Skeleton variant="text" width="75%" height={20} />
        </Stack>
      </Card>
    </Stack>
  );
}
