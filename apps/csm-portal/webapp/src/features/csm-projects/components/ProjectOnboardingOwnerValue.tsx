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

import { Typography } from "@wso2/oxygen-ui";
import type { JSX } from "react";
import UserRefLink from "@components/UserRefLink";
import type { ProjectOnboardingOwnerRef } from "@features/csm-projects/types/csmProjects";

/** A project's onboarding owner as a user link, or "—" when there is none.
 *  Shared by the overview cell and the Onboarding tab. */
export default function ProjectOnboardingOwnerValue({
  owner,
}: {
  owner?: ProjectOnboardingOwnerRef | null;
}): JSX.Element {
  return (
    <Typography variant="body2">
      {owner ? (
        <UserRefLink
          name={owner.name}
          email={owner.email || undefined}
          userId={owner.id}
        />
      ) : (
        "—"
      )}
    </Typography>
  );
}
