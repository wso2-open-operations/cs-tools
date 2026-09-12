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

import { useState } from "react";
import { useLocation, useNavigate } from "react-router-dom";
import type { OxygenTheme } from "@wso2/oxygen-ui/styles/OxygenThemeBase";
import { Box, IconButton, Typography, pxToRem, useTheme } from "@wso2/oxygen-ui";
import { ArrowLeft, Grip } from "@wso2/oxygen-ui-icons-react";
import { goToMyAppsScreen } from "@components/microapp-bridge";
import { ConfirmDialog } from "@components/common/ConfirmDialog";

const ROOT_PATHS = ["/", "/support", "/operations", "/more"];

export function TopBar() {
  const location = useLocation();
  const navigate = useNavigate();
  const isRootPath = ROOT_PATHS.includes(location.pathname);
  const theme = useTheme<OxygenTheme>();

  return (
    <Box
      position="sticky"
      top={0}
      display="flex"
      alignItems="center"
      px={2}
      pb={2}
      sx={{
        // Not a flat pt:7 (customer-portal microapp's own AppBar.tsx uses that) — it ties to the
        // device's actual safe-area inset (--safe-top, set from the native bridge) plus just
        // enough clearance for the ExitButton pill's own height, since a flat value is only
        // correct for whatever inset it was tuned against: on a Dynamic Island phone (~59px
        // inset) pt:7 (56px) undershoots the inset and the pill overflows past this bar's solid
        // background into the page content below it. customer-portal's AppBar doesn't have this
        // problem because it always has title/subtitle content padding the bar out further;
        // this TopBar has no such content, so it needs the calc instead.
        pt: "calc(var(--safe-top, 44px) + 28px)",
        backgroundColor: `${theme.vars.palette.background.default} !important`,
        zIndex: theme.zIndex.appBar,
      }}
    >
      <Box sx={{ minWidth: 32 }}>
        {isRootPath ? (
          <ExitButton />
        ) : (
          <IconButton onClick={() => navigate(-1)} aria-label="Go back" sx={{ p: 0 }} disableRipple>
            <ArrowLeft size={pxToRem(20)} />
          </IconButton>
        )}
      </Box>
    </Box>
  );
}

function ExitButton() {
  const [open, setOpen] = useState(false);

  return (
    <>
      <IconButton
        disableRipple
        color="primary"
        sx={{
          gap: 1,
          position: "absolute",
          // A small fixed clearance on top of the raw inset — flush against --safe-top alone
          // sits the pill right against the status bar/notch with no breathing room on devices
          // where the bridge reports an accurate, tight inset (seen on Android). Still scales
          // with the device's actual safe area rather than a flat offset, so it stays correct
          // across notch depths/status bar heights.
          top: "calc(var(--safe-top, 44px) + 8px)",
          left: 10,
          p: 0,
        }}
        onClick={() => setOpen(true)}
      >
        <Grip size={pxToRem(20)} />
        <Typography>Go to Apps</Typography>
      </IconButton>

      <ConfirmDialog
        open={open}
        title="Return to Apps"
        description="Are you sure you want to leave this application?"
        confirmColor="primary"
        confirmLabel="Leave"
        onClose={() => setOpen(false)}
        onConfirm={goToMyAppsScreen}
      />
    </>
  );
}
