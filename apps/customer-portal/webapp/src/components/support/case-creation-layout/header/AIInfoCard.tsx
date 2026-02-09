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

import { Box, Card, CardContent, colors, Typography } from "@wso2/oxygen-ui";
import { CircleCheck } from "@wso2/oxygen-ui-icons-react";
import type { JSX } from "react";

/**
 * AI Info Card component showing that details are auto-populated.
 *
 * @returns {JSX.Element} The rendered AI info card.
 */
export const AIInfoCard = (): JSX.Element => (
  <Card>
    {/* AI info card wrapper */}
    <CardContent sx={{ display: "flex", gap: 1.5, p: 2 }}>
      <CircleCheck
        size={20}
        color={colors.orange[700]}
        style={{ flexShrink: 0, marginTop: 2 }}
      />
      {/* AI info text container */}
      <Box>
        <Typography variant="body2">
          Case details auto-populated from your conversation
        </Typography>
        <Typography variant="caption">
          All fields below have been filled based on your chat with Novera.
          Please review and edit as needed.
        </Typography>
      </Box>
    </CardContent>
  </Card>
);
