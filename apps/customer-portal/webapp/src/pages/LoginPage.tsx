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

import Actions from "@/components/common/header/Actions";
import Brand from "@/components/common/header/Brand";
import LoginBackground from "@/components/login-page/LoginBackground";
import LoginBox from "@/components/login-page/LoginBox";
import LoginFooter from "@/components/login-page/LoginFooter";
import LoginSlogan from "@/components/login-page/LoginSlogan";
import ParticleBackground from "@/components/login-page/ParticleBackground";
import { Box, Grid, Header as HeaderUI, Paper, Stack } from "@wso2/oxygen-ui";
import { type JSX } from "react";

export default function LoginPage(): JSX.Element {
  return (
    <Box sx={{ height: "100vh", display: "flex", flexDirection: "column" }}>
      <ParticleBackground />
      <HeaderUI>
        <Brand />
        <HeaderUI.Spacer />
        <Actions showUserProfile={false} />
      </HeaderUI>
      <Grid container sx={{ flex: 1 }}>
        <Grid
          size={{ xs: 12, md: 8 }}
          sx={{
            display: "flex",
            alignItems: "flex-start",
            justifyContent: "flex-start",
            padding: 18,
            textAlign: "left",
            position: "relative",
          }}
        >
          <Box>
            <Stack
              direction="column"
              alignItems="start"
              gap={2}
              maxWidth={580}
              display={{ xs: "none", md: "flex" }}
            >
              <LoginSlogan />
            </Stack>
          </Box>
          <LoginBackground />
        </Grid>

        <Grid size={{ xs: 12, md: 4 }}>
          <Paper
            sx={{
              display: "flex",
              padding: 4,
              width: "100%",
              height: "100%",
              flexDirection: "column",
              position: "relative",
              textAlign: "left",
            }}
          >
            <Box
              sx={{
                alignItems: "center",
                justifyContent: "center",
                padding: 4,
                width: "100%",
                maxWidth: 500,
                margin: "auto",
              }}
            >
              <LoginBox />
              <LoginFooter />
            </Box>
          </Paper>
        </Grid>
      </Grid>
    </Box>
  );
}
