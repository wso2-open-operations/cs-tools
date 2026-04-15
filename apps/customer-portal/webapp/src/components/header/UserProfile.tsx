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
import type { JSX } from "react";
import { useState } from "react";
import { useNavigate } from "react-router";
import { useAsgardeo } from "@asgardeo/react";
import { LogOut, User } from "@wso2/oxygen-ui-icons-react";
import useGetUserDetails from "@features/settings/api/useGetUserDetails";
import { useLogger } from "@hooks/useLogger";
import UserProfileModal from "@components/header/UserProfileModal";

/**
 * User profile component.
 *
 * @returns {JSX.Element} The User profile component.
 */
export default function UserProfile(): JSX.Element {
  const navigate = useNavigate();
  const [profileModalOpen, setProfileModalOpen] = useState(false);
  const { signOut, isLoading: isAuthLoading, isSignedIn } = useAsgardeo();
  const { data: userDetails, isLoading, isError } = useGetUserDetails();
  const logger = useLogger();

  if (!isAuthLoading && !isSignedIn) {
    return <></>;
  }

  const handleLogout = async () => {
    try {
      await signOut();
    } catch (error) {
      logger.error("Failed to sign out", error);
    } finally {
      navigate("/");
    }
  };

  const isProfilePending = isLoading || isAuthLoading;
  const useErrorFallback = isError && !userDetails;

  const name = isProfilePending
    ? ""
    : useErrorFallback
      ? "Unknown User"
      : userDetails?.firstName || userDetails?.lastName
        ? `${userDetails.firstName || ""} ${userDetails.lastName || ""}`.trim()
        : "--";

  const email = isProfilePending
    ? "\u00a0"
    : useErrorFallback
      ? "--"
      : userDetails?.email || "--";

  const headerName = isProfilePending ? "Loading…" : name;
  const headerEmail = isProfilePending ? "\u00a0" : email;

  return (
    <>
      <UserMenu>
        <UserMenu.Trigger name={name} />
        <UserMenu.Header name={headerName} email={headerEmail} />
        <UserMenu.Divider />
        <UserMenu.Item
          icon={<User size={18} />}
          label="Profile"
          onClick={() => setProfileModalOpen(true)}
        />
        <UserMenu.Logout
          icon={<LogOut size={18} />}
          label="Log out"
          onClick={handleLogout}
        />
      </UserMenu>
      <UserProfileModal
        open={profileModalOpen}
        onClose={() => setProfileModalOpen(false)}
      />
    </>
  );
}
