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
import { Card, Skeleton, Stack } from "@wso2/oxygen-ui";

export function BubbleUserSkeleton() {
  return (
    <Stack direction="row" justifyContent="end" width="100%">
      <Card
        component={Stack}
        sx={{
          p: 1.5,
          ml: 10,
          width: "fit-content",
          bgcolor: "background.paper",
          borderStyle: "dashed",
          borderWidth: 1,
          borderColor: "divider",
        }}
      >
        <Stack gap={1}>
          <Skeleton variant="text" width={150} height={20} />
          <Skeleton variant="text" width={100} height={20} />
        </Stack>

        <Stack direction="row" justifyContent="end" mt={1}>
          <Skeleton variant="text" width={50} height={20} />
        </Stack>
      </Card>
    </Stack>
  );
}
