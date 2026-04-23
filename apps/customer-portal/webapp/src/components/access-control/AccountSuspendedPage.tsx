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
import suspensionIllustration from "@assets/access-control/project-suspended.svg";

/**
 * Shown in ProjectHub when every project the user has access to is suspended.
 * The user cannot access any portal features.
 *
 * @returns {JSX.Element} Account suspension notice with illustration and copy.
 */
export default function AccountSuspendedPage(): JSX.Element {
  return (
    <Box
      sx={{
        display: "flex",
        flexDirection: { xs: "column", md: "row" },
        alignItems: { xs: "stretch", md: "center" },
        justifyContent: "center",
        gap: { xs: 3, md: 5 },
        flex: 1,
        py: { xs: 6, md: 10 },
        px: 3,
        width: "100%",
        maxWidth: 960,
        mx: "auto",
      }}
    >
      <Stack
        spacing={{ xs: 2.5, md: 3 }}
        sx={{
          flex: 1,
          minWidth: 0,
          textAlign: "left",
          justifyContent: "center",
        }}
      >
        <Typography variant="h4" fontWeight={700}>
          Your account has been suspended!
        </Typography>

        <Typography
          variant="body1"
          color="text.secondary"
          sx={{ lineHeight: 1.75 }}
        >
          Your account has been temporarily suspended because all your
          associated projects are currently inactive. This means you won&apos;t
          be able to access any portal features for now.
        </Typography>

        <Typography
          variant="body1"
          color="text.secondary"
          sx={{ lineHeight: 1.75 }}
        >
          If you need help or have questions about your project status, please
          feel free to reach out to your Account Manager through email.
        </Typography>
      </Stack>

      <Box
        sx={{
          width: { xs: "100%", md: 300 },
          maxWidth: { xs: 320, md: 300 },
          flexShrink: 0,
          alignSelf: { xs: "center", md: "auto" },
        }}
      >
        <Box
          component="img"
          src={suspensionIllustration}
          alt=""
          aria-hidden
          sx={{
            width: "100%",
            height: "auto",
            display: "block",
          }}
        />
      </Box>
    </Box>
  );
}
