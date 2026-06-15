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

import { IconButton, Tooltip } from "@wso2/oxygen-ui";
import { Pin, PinOff } from "@wso2/oxygen-ui-icons-react";
import type { JSX } from "react";
import { useAsgardeo } from "@asgardeo/react";
import { useLocation } from "react-router";
import {
  toggleRecentViewPin,
  useRecentViews,
  useRecordRecentView,
} from "@features/csm-recent/hooks/useRecentViews";
import { currentPageEntry } from "@features/csm-recent/currentPageEntry";

/**
 * Pins the CURRENT route into the top-nav working set — for any page, not just
 * case/project/account detail (dashboards, filtered searches, list pages).
 *
 * If the route is already tracked as a recent (e.g. a detail page recorded it
 * on visit), we pin that existing entry so we never create a duplicate; the
 * richer auto-recorded title wins. Otherwise we derive a page/search entry from
 * the route, record it, and pin it.
 */
export default function PinThisPageButton(): JSX.Element | null {
  const { isSignedIn } = useAsgardeo();
  const location = useLocation();
  const recents = useRecentViews();
  const record = useRecordRecentView();

  if (!isSignedIn) return null;

  const href = location.pathname + location.search;
  const existing = recents.find((e) => e.href === href);
  const isPinned = !!existing?.pinned;

  const onToggle = () => {
    if (existing) {
      toggleRecentViewPin(existing.kind, existing.id);
      return;
    }
    const entry = currentPageEntry(location.pathname, location.search);
    record(entry); // adds to recents (unpinned, most recent)
    toggleRecentViewPin(entry.kind, entry.id); // then promote into the bar
  };

  return (
    <Tooltip
      title={
        isPinned
          ? "Unpin this page from the top nav bar"
          : "Pin this page to the top nav bar"
      }
    >
      <IconButton
        size="small"
        aria-label={isPinned ? "Unpin this page" : "Pin this page to top nav bar"}
        aria-pressed={isPinned}
        onClick={onToggle}
        // Pinned state carries the brand accent, kept AA-legible per scheme
        // (primary.dark on light, primary.main on dark) — palette.mode is
        // unreliable under CssVars.
        sx={(t) => ({
          ...(isPinned
            ? {
                color: t.palette.primary.dark,
                ...t.applyStyles("dark", { color: t.palette.primary.main }),
              }
            : {}),
        })}
      >
        {isPinned ? <PinOff size={18} /> : <Pin size={18} />}
      </IconButton>
    </Tooltip>
  );
}
