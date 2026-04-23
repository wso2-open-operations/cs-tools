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

import { Box, Card, pxToRem, Stack, Typography } from "@wso2/oxygen-ui";
import { User } from "@wso2/oxygen-ui-icons-react";

export function StakeholderItem({ name, role }: { name: string; role: string }) {
  return (
    <Card sx={{ bgcolor: "background.default" }} component={Stack} direction="row" justifyContent="space-between" p={1}>
      <Stack direction="row" gap={1}>
        <Box color="text.secondary">
          <User size={pxToRem(18)} />
        </Box>
        <Typography variant="body1" fontWeight="medium">
          {name}
        </Typography>
      </Stack>
      <Typography variant="body2" fontWeight="regular" color="text.secondary">
        {role}
      </Typography>
    </Card>
  );
}
