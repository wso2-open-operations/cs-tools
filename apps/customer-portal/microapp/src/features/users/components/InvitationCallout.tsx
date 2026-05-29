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
import { alpha, Box, Card, Stack, Typography } from "@wso2/oxygen-ui";
import { Info } from "@wso2/oxygen-ui-icons-react";

export function InvitationCallout() {
  return (
    <Card
      component={Stack}
      direction="row"
      sx={(theme) => ({ bgcolor: alpha(theme.palette.info.main, 0.2), p: 1.5, gap: 2 })}
    >
      <Box sx={{ color: "info.main" }}>
        <Info size={18} />
      </Box>
      <Stack>
        <Typography variant="body2" fontWeight="medium" color="info">
          Direct User Invitation
        </Typography>
        <Typography variant="subtitle2" color="text.secondary">
          Send an email invitation directly to a user to join this project. The invitation link will be valid for 7
          days.
        </Typography>
      </Stack>
    </Card>
  );
}
