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

import { Link, useLocation, useNavigate } from "react-router-dom";
import { Avatar, Box, IconButton, Stack, Typography } from "@wso2/oxygen-ui";
import { ArrowLeft } from "@wso2/oxygen-ui-icons-react";
import { useUserStore } from "@src/store/user";
import { initialsOf } from "@utils/initials";

const ROOT_PATHS = ["/", "/support", "/operations", "/more", "/profile"];

export function TopBar() {
  const location = useLocation();
  const navigate = useNavigate();
  const user = useUserStore((state) => state.user);
  const isProfileActive = location.pathname.startsWith("/profile");
  const showBackButton = !ROOT_PATHS.includes(location.pathname);

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
        {showBackButton ? (
          <IconButton onClick={() => navigate(-1)} aria-label="Go back" size="small">
            <ArrowLeft size={22} />
          </IconButton>
        ) : (
          <Stack direction="row" alignItems="center" gap={1}>
            <img src="/logo-dark.svg" alt="Company Logo" style={{ height: 20, width: "auto" }} />
            <Typography variant="subtitle1" fontWeight={600}>
              CSM Portal
            </Typography>
          </Stack>
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
