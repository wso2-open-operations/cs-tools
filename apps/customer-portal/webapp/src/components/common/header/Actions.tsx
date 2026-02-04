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
  Box,
  Button,
  ColorSchemeToggle,
  Divider,
  Header as HeaderUI,
} from "@wso2/oxygen-ui";
import { Users } from "@wso2/oxygen-ui-icons-react";
import type { JSX } from "react";
import { JOIN_COMMUNITY_URL } from "@/constants/appLayoutConstants";
import UserProfile from "@/components/common/header/UserProfile";

interface ActionsProps {
  showUserProfile?: boolean;
}

/**
 * Actions component for the header.
 *
 * @param {ActionsProps} props - The props for the component.
 * @returns {JSX.Element} The HeaderActions component.
 */
export default function Actions({
  showUserProfile = true,
}: ActionsProps): JSX.Element {
  return (
    <HeaderUI.Actions>
      {/* join community button */}
      <Button
        variant="outlined"
        startIcon={<Users size={20} />}
        color="secondary"
        size="medium"
        href={JOIN_COMMUNITY_URL}
        target="_blank"
        rel="noopener noreferrer"
      >
        Join our community
      </Button>
      {/* theme switcher */}
      <ColorSchemeToggle />
      {/* header action divider */}
      <Divider
        orientation="vertical"
        flexItem
        sx={{
          mx: 1,
          display: { xs: "none", sm: "block" },
          visibility: showUserProfile ? "visible" : "hidden",
        }}
      />
      {showUserProfile ? (
        /* user profile menu */
        <UserProfile />
      ) : (
        /* Placeholder for user profile to prevent layout shift */
        <Box sx={{ width: 40, height: 40 }} />
      )}
    </HeaderUI.Actions>
  );
}
