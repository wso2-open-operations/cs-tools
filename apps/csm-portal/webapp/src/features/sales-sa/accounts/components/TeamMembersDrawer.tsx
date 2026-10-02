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

import {
  Avatar,
  Box,
  CircularProgress,
  Drawer,
  List,
  ListItem,
  ListItemAvatar,
  ListItemText,
  Stack,
  Typography,
} from "@wso2/oxygen-ui";
import { useColorScheme } from "@mui/material/styles";
import { useGetAbtTeamMembers } from "../api/useAccountsApi";

export default function TeamMembersDrawer({
  open,
  onClose,
  teamName,
  teamId,
}: {
  open: boolean;
  onClose: () => void;
  teamName: string;
  teamId: string;
}) {
  const { data, isLoading, error } = useGetAbtTeamMembers(teamId);

  const capitalize = (item: string) => item.charAt(0).toUpperCase() + item.slice(1);
  // theme.palette.mode is not live under oxygen-ui's CSS-variables theme
  // (extendTheme()) — confirmed empirically. useColorScheme() is the hook
  // that actually tracks the live scheme.
  const { mode: colorMode, systemMode } = useColorScheme();
  const isDark = (colorMode === "system" ? systemMode : colorMode) === "dark";
  const roleBadgeColors = isDark
    ? { backgroundColor: "#1a3c3c", color: "#80cbc4" }
    : { backgroundColor: "#e0f7fa", color: "#00796b" };

  return (
    <Drawer anchor="right" open={open} onClose={onClose}>
      <Box sx={{ width: 400, p: 5, mt: 10 }}>
        <Box sx={{ display: "flex", justifyContent: "center", mb: 2.5, textDecoration: "underline" }}>
          <Typography variant="h4" fontWeight="bold">
            {teamName}
          </Typography>
        </Box>
        <Typography variant="h5" gutterBottom>
          Team Members
        </Typography>
        {isLoading ? (
          <Stack alignItems="center" sx={{ mt: 4 }}>
            <CircularProgress size={20} />
          </Stack>
        ) : error ? (
          <Typography variant="body2" color="text.secondary">
            Couldn't load team members.
          </Typography>
        ) : (
          <List>
            {data?.map((member, index) => (
              <ListItem key={index}>
                <ListItemAvatar>
                  <Avatar alt={member.name} src={member.employeeThumbnail} />
                </ListItemAvatar>
                <ListItemText
                  primary={
                    <Box sx={{ display: "flex", alignItems: "center" }}>
                      {member.name.replace(" ⓦ", "")}
                      {member.role && (
                        <Box
                          component="span"
                          sx={{
                            ml: 1,
                            px: 1,
                            py: 0.3,
                            borderRadius: "10px",
                            fontSize: "0.75rem",
                            fontWeight: "bold",
                            ...roleBadgeColors,
                          }}
                        >
                          {capitalize(member.role)}
                        </Box>
                      )}
                    </Box>
                  }
                  secondary={member.email}
                />
              </ListItem>
            ))}
          </List>
        )}
      </Box>
    </Drawer>
  );
}
