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

import { Box, Button, Typography } from "@wso2/oxygen-ui";
import { ArrowLeft } from "@wso2/oxygen-ui-icons-react";
import type { JSX } from "react";
import { Link as RouterLink, useLocation, useParams } from "react-router";
import DirectoryMembersList, {
  type DirectoryMemberFilterKey,
} from "@features/csm-admin/components/DirectoryMembersList";

interface DirectoryMemberPageProps {
  filterKey: DirectoryMemberFilterKey;
  /** Singular noun, e.g. "role". */
  entityNoun: string;
  /** Back's fallback target when `location.state.from` is absent (a direct/
   * shared link), e.g. "/admin/roles". */
  listPath: string;
}

/**
 * Shared shell for a role/group/team's member page: the entity's name
 * (carried as router state by the directory row's link — see
 * `DirectoryEntityTable` — falling back to the raw id for a direct/shared
 * link that arrived without it) plus the member list itself. One shell, three
 * thin per-entity pages (`RoleMembersPage`, `GroupMembersPage`,
 * `TeamMembersPage`) so each still has its own route component to test the
 * filter key against.
 *
 * Back reads `location.state.from`, falling back to `listPath` only when
 * absent (a direct/shared link, or the entity's own directory row, which
 * doesn't set `from` since for that caller `from` and `listPath` are the same
 * place anyway). This used to be hardcoded to `listPath` unconditionally,
 * reasoned as "only ever reached from the directory list" — that stopped
 * being true once `DirectoryEntityChip` (a team/role/group chip rendered on
 * a case, an account, and a user profile) started linking here too, and was
 * reported live as a bug: "Back to Teams" from a case always dropped the
 * caller on the plain Teams directory instead of back on the case. Once the
 * destination is dynamic, the label has to be plain "Back" too (not a
 * destination-specific one) — see this app's own Back-navigation convention.
 */
export default function DirectoryMemberPage({
  filterKey,
  entityNoun,
  listPath,
}: DirectoryMemberPageProps): JSX.Element {
  const { id } = useParams<{ id: string }>();
  const location = useLocation();
  const state = location.state as { name?: string; from?: string } | null;
  const name = state?.name ?? id ?? "";
  const backTarget = state?.from ?? listPath;

  if (!id) {
    return (
      <Box sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
        <Typography variant="h5">Not found</Typography>
        <Typography variant="body2" color="text.secondary">
          No {entityNoun} id was given.
        </Typography>
      </Box>
    );
  }

  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 2.5 }}>
      <Button
        component={RouterLink}
        to={backTarget}
        variant="text"
        size="small"
        startIcon={<ArrowLeft size={16} />}
        sx={{ alignSelf: "flex-start" }}
      >
        Back
      </Button>

      <Box>
        <Typography variant="h5">{name}</Typography>
        <Typography variant="body2" color="text.secondary">
          Members of this {entityNoun}
        </Typography>
      </Box>

      <DirectoryMembersList filterKey={filterKey} entityId={id} entityNoun={entityNoun} />
    </Box>
  );
}
