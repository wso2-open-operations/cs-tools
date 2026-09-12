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

import { Box, IconButton, Tooltip, Typography } from "@wso2/oxygen-ui";
import { RefreshCw } from "@wso2/oxygen-ui-icons-react";
import type { JSX } from "react";
import { useState } from "react";
import { formatRelativeTime } from "@features/csm-dashboard/utils/abtDashboard";
import { useRelativeTimeTick } from "@components/RelativeTime";

interface RefreshButtonProps {
  /** Re-runs the underlying query. Wire to the react-query `refetch`. */
  onRefresh: () => void;
  /** True while a fetch is in flight; disables the control to avoid re-entrancy. */
  isFetching: boolean;
  /** react-query `dataUpdatedAt` (ms). When set, shows a "Last refreshed" hint. */
  updatedAt?: number;
  /** Accessible label / tooltip, e.g. "Refresh assigned cases". */
  label: string;
}

/**
 * Reusable refresh control (icon button + "last refreshed" hint), shared
 * across dashboard widgets, case detail tabs, and list pages so refresh
 * looks and behaves the same everywhere.
 */
export default function RefreshButton({
  onRefresh,
  isFetching,
  updatedAt,
  label,
}: RefreshButtonProps): JSX.Element {
  // The "Last refreshed" hint only appears after the user has manually
  // clicked this control at least once — not from the page's initial data
  // load, which also sets `updatedAt`.
  const [hasManuallyRefreshed, setHasManuallyRefreshed] = useState(false);

  // Re-render exactly when "Last refreshed …" text would next change
  // (adaptive shared scheduler — see RelativeTime.tsx), without requiring
  // another fetch or unrelated state change. `now` is passed explicitly
  // into `formatRelativeTime` below (rather than relying on its internal
  // `Date.now()` default) so the React Compiler's auto-memoization sees it
  // as a dependency and recomputes the label. Only registered with the
  // shared scheduler once the label is actually going to render — before
  // the first manual refresh, the hint is hidden, so there's nothing to
  // tick for.
  const now = useRelativeTimeTick(hasManuallyRefreshed ? updatedAt : null);

  const handleRefreshClick = (): void => {
    setHasManuallyRefreshed(true);
    onRefresh();
  };

  return (
    <Box sx={{ display: "flex", alignItems: "center", gap: 1 }}>
      {hasManuallyRefreshed && updatedAt ? (
        <Typography variant="caption" color="text.secondary">
          Last refreshed{" "}
          {formatRelativeTime(new Date(updatedAt).toISOString(), now)}
        </Typography>
      ) : null}
      <Tooltip title={label}>
        {/* span wrapper so the tooltip still shows while the button is disabled */}
        <span>
          <IconButton
            size="small"
            onClick={handleRefreshClick}
            disabled={isFetching}
            aria-label={label}
          >
            <RefreshCw size={14} />
          </IconButton>
        </span>
      </Tooltip>
    </Box>
  );
}
