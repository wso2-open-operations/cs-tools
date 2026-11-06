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

import { type JSX } from "react";
import { Outlet } from "react-router";
import { Skeleton } from "@wso2/oxygen-ui";
import Error403Page from "@components/error/Error403Page";
import { useCurrentUser } from "@context/current-user/CurrentUserContext";

function isAdmin(roles: string[] | undefined): boolean {
  return (roles ?? []).some((role) => role.toLowerCase() === "admin");
}

/** Hides /admin/banners from anyone who is not an admin. The tab filter in CsmAdminLayout is the same check. */
export default function CsmBannersRouteGuard(): JSX.Element {
  const { user, isLoading, isError } = useCurrentUser();
  if (isLoading && !isError) {
    return <Skeleton variant="rounded" height={200} />;
  }
  if (!isAdmin(user?.roles)) {
    return <Error403Page />;
  }
  return <Outlet />;
}
