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

import { type JSX } from "react";
import { Box, Stack, Typography } from "@wso2/oxygen-ui";

/**
 * Shows a dedicated portal-access message when `/users/me` is unauthorized.
 *
 * @returns {JSX.Element} Centered portal access required state.
 */
export default function PortalAccessRequiredPage(): JSX.Element {
  return (
    <Box
      sx={{
        minHeight: "100%",
        display: "flex",
        alignItems: "center",
        justifyContent: "center",
        px: 3,
      }}
    >
      <Stack
        spacing={2}
        alignItems="center"
        sx={{ maxWidth: 760, textAlign: "center", px: { xs: 1, sm: 3 } }}
      >
        <Typography variant="h2">Portal Access Required</Typography>
        <Typography variant="h5" color="text.secondary">
          You've signed in successfully, but your account does not have access to the
          customer portal.
        </Typography>
        <Typography variant="body1" color="text.secondary">
          To gain access, please contact your account manager or reach out to{" "}
          <Box
            component="a"
            href="mailto:support@wso2.com"
            sx={{
              color: "primary.main",
              textDecoration: "none",
              "&:hover": {
                textDecoration: "underline",
              },
            }}
          >
            support@wso2.com
          </Box>
        </Typography>
      </Stack>
    </Box>
  );
}

