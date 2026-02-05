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

import { Skeleton, UserMenu } from "@wso2/oxygen-ui";
import type { JSX } from "react";
import { useNavigate } from "react-router";
import { useAsgardeo } from "@asgardeo/react";
import { useMockConfig } from "@/providers/MockConfigProvider";
import { User } from "@wso2/oxygen-ui-icons-react";
import { useLogger } from "@/hooks/useLogger";
import useGetUserDetails from "@/api/useGetUserDetails";
import ErrorIndicator from "@/components/common/errorIndicator/ErrorIndicator";

/**
 * User profile component.
 *
 * @returns {JSX.Element} The User profile component.
 */
export default function UserProfile(): JSX.Element {
  const navigate = useNavigate();
  const { signOut, isLoading: isAuthLoading, isSignedIn } = useAsgardeo();
  const { isMockEnabled } = useMockConfig();
  const { data: userDetails, isLoading, isError } = useGetUserDetails();
  const logger = useLogger();

  if (!isAuthLoading && !isSignedIn && !isMockEnabled) {
    return <></>;
  }
  const user = userDetails
    ? {
        name:
          userDetails.firstName || userDetails.lastName
            ? `${userDetails.firstName || ""} ${userDetails.lastName || ""}`.trim()
            : "--",
        email: userDetails.email || "--",
        avatar:
          userDetails.firstName || userDetails.lastName ? (
            `${(userDetails.firstName?.[0] || "").toUpperCase()}${(
              userDetails.lastName?.[0] || ""
            ).toUpperCase()}`
          ) : (
            <User />
          ),
      }
    : null;

  // Loading user object with skeletons
  const loadingUser = {
    name: <Skeleton variant="text" width={100} />,
    email: <Skeleton variant="text" width={150} />,
    avatar: <></>,
  };

  // Error user object with tooltips
  const errorUser = {
    name: <ErrorIndicator entityName="user" />,
    avatar: <User />,
  };

  const userToRender =
    isLoading || isAuthLoading
      ? loadingUser
      : isError
        ? errorUser
        : user || errorUser;

  return (
    <>
      {/* user profile menu */}
      <UserMenu
        user={userToRender as any}
        onProfileClick={() => logger.debug("Profile clicked")}
        onSettingsClick={() => logger.debug("Settings clicked")}
        onLogout={async () => {
          await signOut();
          navigate("/");
        }}
      />
    </>
  );
}
