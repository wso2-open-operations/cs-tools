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

import { Box, Chip, Tooltip } from "@wso2/oxygen-ui";
import type { JSX } from "react";
import { useAsgardeo } from "@asgardeo/react";
import { useLocation } from "react-router";
import {
  toggleRecentViewPin,
  useRecentViews,
  type RecentView,
} from "@features/csm-recent/hooks/useRecentViews";
import { kindIcon } from "@features/csm-recent/kindMeta";
import { useNavTransition } from "@hooks/useNavTransition";

/** Compact chip label: the case id / entity name (the part before " · "). */
function shortLabel(entry: RecentView): string {
  return entry.title.split(" · ")[0] || entry.title;
}

/**
 * The pinned working set rendered as persistent tabs in the top nav bar, so an
 * engineer can hold several cases (or projects/accounts) one click away without
 * leaving the current screen. Pins are added from the Recent Views panel and
 * removed via each tab's delete affordance.
 *
 * Sits in the header's flexible middle region: when nothing is pinned it
 * collapses to a plain spacer so the brand stays left and the actions stay
 * right, exactly like the `Header.Spacer` it replaces.
 */
export default function PinnedTabs(): JSX.Element {
  const { isSignedIn } = useAsgardeo();
  const navigate = useNavTransition();
  const location = useLocation();
  const pinned = useRecentViews().filter((e) => e.pinned);

  // Always occupy the flexible middle slot so the layout is identical whether or
  // not anything is pinned (and when signed out, where pins are not actionable).
  if (!isSignedIn || pinned.length === 0) {
    return <Box sx={{ flexGrow: 1 }} />;
  }

  return (
    <Box
      sx={{
        flexGrow: 1,
        minWidth: 0,
        display: "flex",
        alignItems: "center",
        gap: 0.75,
        overflowX: "auto",
        // Quiet scrollbar — the strip should read as content, not a control.
        "&::-webkit-scrollbar": { height: 6 },
        "&::-webkit-scrollbar-thumb": {
          bgcolor: "action.disabled",
          borderRadius: 3,
        },
      }}
    >
      {pinned.map((entry) => {
        // A search pin is path + query, so match the full href; for everything
        // else match the path alone (the case/project/account a tab points at,
        // independent of any query). Path-only matching would light up every
        // pinned search on the same route at once.
        const active =
          entry.kind === "search"
            ? location.pathname + location.search === entry.href
            : location.pathname === entry.href.split("?")[0];
        return (
          <Tooltip key={`${entry.kind}-${entry.id}`} title={entry.title}>
            <Chip
              size="small"
              icon={kindIcon(entry.kind, 14)}
              label={shortLabel(entry)}
              variant={active ? "filled" : "outlined"}
              onClick={() => navigate(entry.href)}
              onDelete={() => toggleRecentViewPin(entry.kind, entry.id)}
              aria-label={`${shortLabel(entry)} — pinned tab`}
              sx={{
                flexShrink: 0,
                maxWidth: 200,
                cursor: "pointer",
                // Active = where you are now: a calm selected fill, not the
                // brand orange (white-on-orange small text fails AA).
                ...(active
                  ? { bgcolor: "action.selected", fontWeight: 600 }
                  : {}),
              }}
            />
          </Tooltip>
        );
      })}
    </Box>
  );
}
