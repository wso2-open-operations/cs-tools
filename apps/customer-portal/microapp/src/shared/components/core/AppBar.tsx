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
import { AppBar as MuiAppBar, Stack, Toolbar, Typography } from "@wso2/oxygen-ui";

import { useLayout } from "@context/layout";
import { useThemeMode } from "@context/theme";

import { BackButton, ExitButton, ProjectSelect } from "@shared/components/core";

import { useAppBarHeight, useNavigation } from "@shared/hooks";

export function AppBar() {
  const mode = useThemeMode();
  const { title, visibility, slots } = useLayout();
  const { ref } = useAppBarHeight();
  const { back } = useNavigation();

  return (
    <MuiAppBar
      ref={ref}
      position="sticky"
      elevation={0}
      sx={{
        backgroundColor: `${mode === "light" ? "white" : "black"} !important`,
        position: "sticky",
        pt: "var(--safe-top)",
      }}
    >
      <Toolbar
        disableGutters
        sx={{
          flexDirection: "column",
          alignItems: "stretch",
          height: "auto",
          minHeight: "fit-content",
        }}
      >
        {visibility?.exitButton && (
          <Stack direction="row">
            <ExitButton />
          </Stack>
        )}

        <Stack direction="row" justifyContent="space-between" alignItems="center" gap={1} pb={1}>
          <Stack direction="row" alignItems="center" gap={1} flex={1}>
            {visibility?.backAction && <BackButton onClick={back} />}

            {slots?.leading}

            <Stack flex={1}>
              {slots?.overline && (
                <Typography component="div" variant="body2" fontWeight="regular" color="text.secondary">
                  {slots.overline}
                </Typography>
              )}

              {title && (
                <Typography variant="h6" fontWeight="medium">
                  {title}
                </Typography>
              )}

              {slots?.subtitle && (
                <Typography component="div" variant="subtitle2" fontWeight="regular" color="text.secondary">
                  {slots.subtitle}
                </Typography>
              )}
            </Stack>
          </Stack>

          {slots?.trailing}
        </Stack>

        {visibility?.projectSelector && <ProjectSelect />}

        {slots?.bottom}
      </Toolbar>
    </MuiAppBar>
  );
}
