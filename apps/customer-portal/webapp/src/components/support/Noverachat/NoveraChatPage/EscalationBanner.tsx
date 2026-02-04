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

import { Box, Button, Paper, Typography } from "@wso2/oxygen-ui";
import { CircleAlert } from "@wso2/oxygen-ui-icons-react";
import { type JSX } from "react";

/**
 * Interface for EscalationBanner component.
 */
interface EscalationBannerProps {
  visible: boolean;
}

/**
 * EscalationBanner component for NoveraChatPage.
 */
export default function EscalationBanner({
  visible,
}: EscalationBannerProps): JSX.Element | null {
  if (!visible) return null;

  return (
    <Paper
      sx={{
        mb: 2,
        py: 1,
        px: 1.5,
        display: "flex",
        alignItems: "center",
        gap: 1,
      }}
    >
      <CircleAlert size={18} color="#ea580c" style={{ flexShrink: 0 }} />

      <Box sx={{ flex: 1 }}>
        <Typography variant="body2">
          Need more help? I can create a support case with all the details
          we&apos;ve discussed.
        </Typography>
      </Box>

      <Button variant="outlined" size="small" color="warning">
        Create Case
      </Button>
    </Paper>
  );
}
