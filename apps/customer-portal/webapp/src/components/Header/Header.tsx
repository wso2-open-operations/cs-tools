// Copyright (c) 2025 WSO2 LLC. (https://www.wso2.com).
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

import { Box, IconButton } from "@mui/material";
import { useState } from "react";

import { Wso2Logo } from "../../assets/icons/side-nav-bar-icons";
import SearchBar from "./SearchBar/SearchBar";
import Profile from "./Profile/Profile";
import Announcements from "./Announcements/Announcements";
import Community from "./Community/Community";
import DropdownMenu from "./ProjectSwitcherDropdown/DropdownMenu";
import ProfileMenu from "../ProfileMenu/ProfileMenu";
import { Notifications } from "@mui/icons-material";
import { useUserQuery, getInitialsFromName } from "../../hooks/useUser";
import { useAppAuthContext } from "../../providers/AppAuthProvider";

interface HeaderProps {
  variant?: "home" | "dashboard";
  projectName?: string;
  projectKey?: string;
  currentProjectId?: string;
  onProjectChange?: (projectId: string) => void;
}

const Header = ({
  variant = "home",
  projectName = "Dreamworks Inc",
  projectKey,
  currentProjectId,
  onProjectChange,
}: HeaderProps) => {
  const { data: user } = useUserQuery();
  const { appSignOut } = useAppAuthContext();
  const [profileMenuAnchor, setProfileMenuAnchor] =
    useState<null | HTMLElement>(null);

  const userInitials = user ? getInitialsFromName(user.name) : "U";
  const profilePictureUrl = user?.picture;

  const handleProfileClick = (event: React.MouseEvent<HTMLElement>) => {
    setProfileMenuAnchor(event.currentTarget);
  };

  const handleProfileMenuClose = () => {
    setProfileMenuAnchor(null);
  };

  const handleProjectChange = (
    projectId: string,
    _projectName: string,
    _projectKey: string
  ) => {
    if (onProjectChange) {
      onProjectChange(projectId);
    }
  };
  if (variant === "dashboard") {
    return (
      <Box
        component="header"
        sx={{
          backgroundColor: "background.paper",
          borderBottom: (theme) => `1px solid ${theme.palette.grey[200]}`,
          px: 3,
          py: 2,
          display: "flex",
          alignItems: "center",
          gap: 2,
          zIndex: 10,
        }}
      >
        {/* Project Selector */}
        <DropdownMenu
          currentProjectName={projectName}
          currentProjectKey={projectKey}
          currentProjectId={currentProjectId}
          onProjectChange={handleProjectChange}
        />

        {/* Search Bar */}
        <Box sx={{ flex: 1, maxWidth: "448px" }}>
          <SearchBar placeholder="Search cases, tickets, or users..." />
        </Box>

        {/* Right Section */}
        <Box display="flex" alignItems="center" gap={1.5} ml="auto">
          <Community label="Join our community" onClick={() => {}} />

          <IconButton
            sx={{
              position: "relative",
              "&:hover": {
                bgcolor: (theme) => theme.palette.grey[100],
              },
            }}
          >
            <Notifications sx={{ fontSize: "1.25rem" }} />
            <Box
              sx={{
                position: "absolute",
                top: 8,
                right: 8,
                width: 8,
                height: 8,
                bgcolor: "primary.dark",
                borderRadius: "50%",
              }}
            />
          </IconButton>

          <Profile
            initials={userInitials}
            profilePictureUrl={profilePictureUrl}
            onClick={handleProfileClick}
          />
        </Box>

        <ProfileMenu
          anchorEl={profileMenuAnchor}
          open={Boolean(profileMenuAnchor)}
          onClose={handleProfileMenuClose}
          userName={user?.name || "User"}
          userEmail={user?.email}
          onLogout={appSignOut}
        />
      </Box>
    );
  }

  // Home variant (original header)
  return (
    <Box
      component="header"
      sx={{
        backgroundColor: "background.paper",
        borderBottom: (theme) => `1px solid ${theme.palette.grey[200]}`,
        px: "24px",
        py: "8px",
        display: "flex",
        justifyContent: "center",
        alignItems: "center",
        gap: 4,
        zIndex: 2,
      }}
    >
      {/* WSO2 Logo */}
      <Box sx={{ display: "flex", alignItems: "center", maxWidth: 280 }}>
        <Wso2Logo width={82} height="auto" />
      </Box>

      {/* Search Bar */}
      <SearchBar placeholder="Search Projects..." />

      {/* Community */}
      <Community label="Join our community" onClick={() => {}} />

      {/* Announcements */}
      <Announcements count={2} onClick={() => {}} />

      {/* Profile Avatar */}
      <Profile
        initials={userInitials}
        profilePictureUrl={profilePictureUrl}
        onClick={handleProfileClick}
      />

      <ProfileMenu
        anchorEl={profileMenuAnchor}
        open={Boolean(profileMenuAnchor)}
        onClose={handleProfileMenuClose}
        userName={user?.name || "User"}
        userEmail={user?.email}
        onLogout={appSignOut}
      />
    </Box>
  );
};

export default Header;
