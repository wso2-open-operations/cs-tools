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
import { Link, useLocation, useNavigate } from "react-router-dom";
import { Avatar, Box, IconButton, Typography } from "@wso2/oxygen-ui";
import { ArrowLeft, Grip } from "@wso2/oxygen-ui-icons-react";
import { useUserStore } from "@src/store/user";
import { goToMyAppsScreen } from "@components/microapp-bridge";
import { ConfirmDialog } from "@components/common/ConfirmDialog";
import { initialsOf } from "@utils/initials";

const ROOT_PATHS = ["/", "/support", "/operations", "/more", "/profile"];

export function TopBar() {
  const location = useLocation();
  const navigate = useNavigate();
  const user = useUserStore((state) => state.user);
  const isProfileActive = location.pathname.startsWith("/profile");
  const isRootPath = ROOT_PATHS.includes(location.pathname);

  return (
    <Box
      position="sticky"
      top={0}
      bgcolor="background.paper"
      display="flex"
      justifyContent="space-between"
      alignItems="center"
      px={2}
      pb={1}
      sx={{
        pt: 7,
        borderBottom: "1px solid",
        borderColor: "divider",
        zIndex: (theme) => theme.zIndex.appBar,
      }}
    >
      <Box sx={{ minWidth: 32 }}>
        {isRootPath ? (
          <ExitButton />
        ) : (
          <IconButton onClick={() => navigate(-1)} aria-label="Go back" size="small">
            <ArrowLeft size={22} />
          </IconButton>
        )}
      </Box>

      <IconButton component={Link} to="/profile" aria-label="Profile" size="small">
        <Avatar
          src={user?.avatarUrl}
          slotProps={{ img: { referrerPolicy: "no-referrer" } }}
          sx={{
            width: 28,
            height: 28,
            fontSize: 13,
            bgcolor: isProfileActive ? "primary.main" : "grey.400",
          }}
        >
          {initialsOf(user?.name ?? "")}
        </Avatar>
      </IconButton>
    </Box>
  );
}

// Leaves the microapp WebView and returns to the native shell's app switcher — mirrors the
// customer-portal microapp's own ExitButton (components/core/AppBar.tsx), shown on root pages in
// place of the back button. Floats in the safe-area strip above the main row (same technique the
// customer-portal version uses: absolutely positioned at `var(--safe-top)`), so it doesn't compete
// with page content for width on narrow phones.
function ExitButton() {
  const [open, setOpen] = useState(false);

  return (
    <>
      <IconButton
        disableRipple
        color="error"
        sx={{
          gap: 1,
          position: "absolute",
          top: "var(--safe-top)",
          left: 10,
          p: 0,
        }}
        onClick={() => setOpen(true)}
      >
        <Grip size={20} />
        <Typography variant="body2" fontWeight={600}>
          Go to Apps
        </Typography>
      </IconButton>

      <ConfirmDialog
        open={open}
        title="Return to Apps"
        description="Are you sure you want to leave this application?"
        confirmColor="error"
        confirmLabel="Leave"
        onClose={() => setOpen(false)}
        onConfirm={goToMyAppsScreen}
      />
    </>
  );
}
