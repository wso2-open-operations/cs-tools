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

import { Box, Button, CircularProgress, Typography } from "@wso2/oxygen-ui";
import { Download } from "@wso2/oxygen-ui-icons-react";
import type { JSX } from "react";
import type { ResolvedAudienceProject } from "@features/csm-announcements/api/useResolveAnnouncementAudience";
import { downloadAudienceCsv } from "@features/csm-announcements/utils/audienceCsv";

interface ResolvedAudienceListProps {
  projects: ResolvedAudienceProject[];
  total: number;
  isLoading: boolean;
  isError: boolean;
}

/**
 * Live preview of the "all customer projects" scope's resolved audience —
 * count, a scrollable project list, and a CSV export, per the Phase 1
 * brief's "resolved recipient list before send." This is the review step
 * that stands in for the Account Life Cycle filter the platform doesn't
 * have yet (see AudienceScopeControls' own doc comment): the engineer scans
 * this list, not just a count, before submitting.
 */
export default function ResolvedAudienceList({
  projects,
  total,
  isLoading,
  isError,
}: ResolvedAudienceListProps): JSX.Element {
  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 1 }}>
      <Box sx={{ display: "flex", alignItems: "center", justifyContent: "space-between", gap: 1 }}>
        <Typography variant="body2">
          {isLoading ? (
            "Resolving audience…"
          ) : isError ? (
            "Couldn't resolve the audience. Try again."
          ) : (
            <>
              <strong>{total}</strong> project{total === 1 ? "" : "s"} match{total === 1 ? "es" : ""}{" "}
              this scope
            </>
          )}
        </Typography>
        <Button
          size="small"
          variant="outlined"
          startIcon={<Download size={14} />}
          disabled={isLoading || isError || total === 0}
          onClick={() =>
            downloadAudienceCsv(projects, `announcement-audience-${new Date().toISOString().slice(0, 10)}.csv`)
          }
        >
          Export CSV
        </Button>
      </Box>

      <Box
        sx={{
          border: 1,
          borderColor: "divider",
          borderRadius: 1,
          maxHeight: 220,
          overflowY: "auto",
        }}
      >
        {isLoading && (
          <Box sx={{ display: "flex", justifyContent: "center", py: 3 }}>
            <CircularProgress size={20} />
          </Box>
        )}
        {!isLoading && !isError && total === 0 && (
          <Typography variant="body2" color="text.secondary" sx={{ p: 2 }}>
            No projects match these filters.
          </Typography>
        )}
        {!isLoading &&
          !isError &&
          projects.map((p) => (
            <Box
              key={p.id}
              sx={{
                display: "flex",
                justifyContent: "space-between",
                gap: 1,
                px: 1.5,
                py: 0.75,
                borderBottom: 1,
                borderColor: "divider",
                "&:last-of-type": { borderBottom: 0 },
              }}
            >
              <Typography variant="body2" noWrap sx={{ minWidth: 0 }}>
                {p.name}
                {p.accountName ? ` — ${p.accountName}` : ""}
              </Typography>
              <Typography variant="body2" color="text.secondary" sx={{ flex: "none" }}>
                {p.key}
              </Typography>
            </Box>
          ))}
      </Box>
    </Box>
  );
}
