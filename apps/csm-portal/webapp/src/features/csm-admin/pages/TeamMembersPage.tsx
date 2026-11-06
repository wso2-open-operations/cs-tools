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

import { Box } from "@wso2/oxygen-ui";
import type { JSX } from "react";
import { useLocation, useParams } from "react-router";
import { useCurrentUser } from "@context/current-user/CurrentUserContext";
import { usePortalAccess } from "@context/current-user/usePortalAccess";
import DirectoryMemberPage from "@features/csm-admin/components/DirectoryMemberPage";
import TeamChatSpaceCard from "@features/csm-admin/components/TeamChatSpaceCard";
import TeamLeadCard from "@features/csm-admin/components/TeamLeadCard";
import TeamMembersManageCard from "@features/csm-admin/components/TeamMembersManageCard";
import { useGetTeamMembers } from "@features/csm-admin/api/useTeamMembers";
import { useTeamEntityId } from "@features/csm-admin/hooks/useTeamEntityId";
import { callerIsTeamLead } from "@features/csm-admin/utils/teamLeadAccess";

/**
 * A team's member list (`/admin/teams/:id`), filtered by `teamIds`. The `:id`
 * here is usually the team's registry key (e.g. "alpha"), not a UUID — see
 * `DirectoryMembersList`, which still uses it exactly that way below.
 *
 * On top of that unchanged list, this page also resolves the real backing
 * UUID (`useTeamEntityId`) and shows two admin/lead-only sections: the
 * team's own Chat space link (admin or this team's own lead) and the team
 * lead itself (admin only, since reassigning it also changes who
 * entity-service's incident call-escalation ladder pages for this team).
 * An admin also gets "Manage members" (add, remove, change role/alert tier,
 * move), keyed by the registry key `:id` as the team-schedule routes are.
 * None of these sections render at all for anyone else — see each card's own doc
 * comment for why this is a deliberate exception to the show-and-let-the-
 * backend-reject default this app's CLAUDE.md otherwise documents for
 * role/group/team member pages.
 */
export default function TeamMembersPage(): JSX.Element {
  const { id } = useParams<{ id: string }>();
  const location = useLocation();
  const state = location.state as { name?: string } | null;

  const { isAdmin } = usePortalAccess();
  const { user } = useCurrentUser();
  const { teamEntityId } = useTeamEntityId(id, state?.name);

  // Only fetched here to decide "is the caller this team's lead" -- skipped
  // for an admin, who doesn't need that check, and for TeamLeadCard, which
  // fetches the identical roster itself (same query key, so this doesn't
  // double the request when both render).
  const { data: members } = useGetTeamMembers(isAdmin ? undefined : teamEntityId);
  const isLead = !isAdmin && callerIsTeamLead(members, user?.email);
  const canManageChatSpace = isAdmin || isLead;

  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 2.5 }}>
      {teamEntityId && (canManageChatSpace || isAdmin) && (
        <Box sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
          {canManageChatSpace && (
            <TeamChatSpaceCard
              teamEntityId={teamEntityId}
              canManage={canManageChatSpace}
            />
          )}
          {isAdmin && <TeamLeadCard teamEntityId={teamEntityId} isAdmin={isAdmin} />}
        </Box>
      )}

      {isAdmin && id && <TeamMembersManageCard teamKey={id} isAdmin={isAdmin} />}

      <DirectoryMemberPage filterKey="teamIds" entityNoun="team" listPath="/admin/teams" />
    </Box>
  );
}
