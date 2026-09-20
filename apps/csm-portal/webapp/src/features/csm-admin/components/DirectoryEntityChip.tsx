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

import { Chip } from "@wso2/oxygen-ui";
import type { JSX, KeyboardEvent, MouseEvent } from "react";
import { useLocation } from "react-router";
import { useNavTransition } from "@hooks/useNavTransition";

interface DirectoryEntityChipProps {
  /** Registry key / id of the role, group or team, e.g. "agent" or "alpha". */
  id: string;
  /** Display label for the chip. */
  name: string;
  /** e.g. "/admin/roles", "/admin/groups", "/admin/teams" — the id is appended. */
  routeBase: string;
  color?: "default" | "primary";
  variant?: "outlined" | "filled";
}

/**
 * A role/group/team name rendered as a clickable chip navigating to its
 * directory member page (`${routeBase}/${id}`), carrying the display name as
 * router state — the same convention `DirectoryEntityTable` uses for the
 * directory pages themselves (see `DirectoryMemberPage`), so the member page
 * can show the name immediately without a second lookup, falling back to the
 * raw id when the state is unavailable (a direct/shared link).
 *
 * Also carries `from` (this chip's own current location), so `DirectoryMemberPage`'s
 * Back button returns here — a case, an account, a user profile — instead of
 * always dropping the caller on the plain directory list. Reported live as a
 * bug ("Back to Teams" from a case always went to the Teams directory) before
 * this was added; the identical fix already exists for `CsmAdminLayout.tsx`/
 * `CsmUsersPage.tsx`, just hadn't been propagated to this sibling.
 *
 * Stops the click (and Enter/Space activation the underlying `Chip` already
 * wires up) from bubbling, so this can be nested inside a clickable table row
 * (e.g. the users list) without also triggering the row's own navigation.
 */
export default function DirectoryEntityChip({
  id,
  name,
  routeBase,
  color = "default",
  variant = "outlined",
}: DirectoryEntityChipProps): JSX.Element {
  const navigate = useNavTransition();
  const location = useLocation();

  const go = (): void => {
    const from = `${location.pathname}${location.search}${location.hash}`;
    navigate(`${routeBase}/${encodeURIComponent(id)}`, { state: { name, from } });
  };

  return (
    <Chip
      size="small"
      label={name}
      color={color}
      variant={variant}
      clickable
      onClick={(e: MouseEvent) => {
        e.stopPropagation();
        go();
      }}
      // `Chip`'s own clickable behaviour already turns Enter/Space keydowns
      // into a click, but the keydown event itself still bubbles first — stop
      // it here too, or a parent clickable row (see the users list) would
      // also navigate itself on the same keypress.
      onKeyDown={(e: KeyboardEvent) => {
        if (e.key === "Enter" || e.key === " ") {
          e.stopPropagation();
        }
      }}
    />
  );
}
