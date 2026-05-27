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

import { UserMenu } from "@wso2/oxygen-ui";
import { LogOut, User } from "@wso2/oxygen-ui-icons-react";
import { type JSX, useState } from "react";
import { useAsgardeo } from "@asgardeo/react";
import { useLogger } from "@hooks/useLogger";
import { useIdTokenClaims } from "@hooks/useIdTokenClaims";
import UserProfileModal from "@components/header/UserProfileModal";
import { resolveUserInfo } from "@utils/userClaims";

export default function UserProfile(): JSX.Element {
  const { signOut } = useAsgardeo();
  const claims = useIdTokenClaims();
  const logger = useLogger();
  const [profileModalOpen, setProfileModalOpen] = useState(false);

  const handleSignOut = async () => {
    window.dispatchEvent(new CustomEvent("app:signing-out"));
    try {
      await signOut();
    } catch (err) {
      logger.error("Failed to sign out", err);
    }
  };

  const info = resolveUserInfo(claims);

  return (
    <>
      <UserMenu>
        <UserMenu.Trigger name={info.fullName} avatar={info.avatarUrl} />
        <UserMenu.Header
          name={info.fullName}
          email={info.email}
          avatar={info.avatarUrl}
        />
        <UserMenu.Divider />
        <UserMenu.Item
          icon={<User size={16} />}
          label="Profile"
          onClick={() => setProfileModalOpen(true)}
        />
        <UserMenu.Logout
          icon={<LogOut size={16} />}
          label="Sign out"
          onClick={handleSignOut}
        />
      </UserMenu>
      <UserProfileModal
        open={profileModalOpen}
        onClose={() => setProfileModalOpen(false)}
      />
    </>
  );
}
