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
import { fromNow } from "@utils/dateTime";

interface RefreshButtonProps {
  /** Re-runs the widget's query. Wire to the react-query `refetch`. */
  onRefresh: () => void;
  /** True while a fetch is in flight; disables the control to avoid re-entrancy. */
  isFetching: boolean;
  /** react-query `dataUpdatedAt` (ms). When set, shows a "Last refreshed" hint. */
  updatedAt?: number;
  /** Accessible label / tooltip, e.g. "Refresh assigned cases". */
  label: string;
}

// Reusable dashboard-widget refresh control — mirrors the webapp's RefreshButton
// (apps/csm-portal/webapp/src/features/csm-dashboard/components/RefreshButton.tsx):
// icon button + "Last refreshed X ago" hint, so refresh looks and behaves the same across apps.
export function RefreshButton({ onRefresh, isFetching, updatedAt, label }: RefreshButtonProps) {
  return (
    <Box sx={{ display: "flex", alignItems: "center", gap: 1 }}>
      {updatedAt ? (
        <Typography variant="caption" color="text.secondary">
          Last refreshed {fromNow(new Date(updatedAt))}
        </Typography>
      ) : null}
      <Tooltip title={label}>
        {/* span wrapper so the tooltip still shows while the button is disabled */}
        <span>
          <IconButton size="small" onClick={onRefresh} disabled={isFetching} aria-label={label}>
            <RefreshCw size={14} />
          </IconButton>
        </span>
      </Tooltip>
    </Box>
  );
}
