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
import { Box, Link, Stack, Typography } from "@wso2/oxygen-ui";
import illustration from "@assets/access-control/portal-access-required.svg";

export default function PortalAccessRequiredPage(): JSX.Element {
  return (
    <Box
      sx={{
        display: "flex",
        justifyContent: "center",
        alignItems: "center",
        mt: 10,
        px: 3,
      }}
    >
      <Stack spacing={2} alignItems="center">
        <Typography variant="h4" fontWeight={600} textAlign="center">
          Portal Access Required
        </Typography>

        <Stack spacing={4} alignItems="center" sx={{ maxWidth: 640 }}>
          <Box
            component="img"
            src={illustration}
            alt="Access denied illustration"
            sx={{
              width: "100%",
              maxWidth: 480,
              height: "auto",
            }}
          />

          <Typography
            variant="body1"
            color="text.secondary"
            textAlign="center"
            sx={{ lineHeight: 1.75 }}
          >
            You've signed in successfully, but your account does not have access
            to the customer portal. To gain access, please contact your account
            manager or reach out to{" "}
            <Link
              href="mailto:support@wso2.com"
              underline="hover"
              sx={{ fontWeight: 500 }}
            >
              support@wso2.com
            </Link>
            .
          </Typography>
        </Stack>
      </Stack>
    </Box>
  );
}
