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
import { Box, Card, Stack, Typography } from "@wso2/oxygen-ui";
import { Clock4 } from "@wso2/oxygen-ui-icons-react";

export function InvitationExpiryCallout() {
  return (
    <Card
      component={Stack}
      direction="row"
      alignItems="center"
      px={2}
      py={1.5}
      gap={2}
      sx={{ bgcolor: "components.popover.state.active.background" }}
    >
      <Box sx={{ color: "primary.main" }}>
        <Clock4 size={20} />
      </Box>
      <Typography variant="subtitle2" fontWeight="medium" color="text.secondary">
        Important: &nbsp;
        <Typography component="span" variant="subtitle2" fontWeight="regular">
          Invitation links expire after 7 days. If the user doesn't accept the invitation within this timeframe, you'll
          need to send a new invitation.
        </Typography>
      </Typography>
    </Card>
  );
}
