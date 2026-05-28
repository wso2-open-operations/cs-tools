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
import { useNavigate } from "react-router-dom";

import type { User } from "@features/users/types";

import type { CaseType } from "@shared/types";

export const useCoreNavigation = () => {
  const navigate = useNavigate();

  return {
    back: () => navigate(-1),

    toHome: (options: { replace: boolean } = { replace: false }) => navigate("/", options),
    toSupport: () => navigate("/support"),
    toUsers: () => navigate("/users"),
    toProfile: () => navigate("/profile"),

    toAll: (type: CaseType) =>
      navigate({
        pathname: "/support/all",
        search: new URLSearchParams({ type }).toString(),
      }),

    toInviteUser: () => navigate("/users/invite"),
    toEditUser: (user: User) => navigate("/users/edit", { state: { user } }),
  };
};
