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

import type { JSX } from "react";
import { Box, Button, Dialog, DialogActions, DialogContent, DialogTitle } from "@wso2/oxygen-ui";
import { Link as RouterLink } from "react-router";
import RelativeTime from "@components/RelativeTime";
import SemanticChip, { type SemanticRole } from "@components/SemanticChip";
import { useGetSmartAlert } from "@features/csm-cases/api/useSnLinkEntities";
import {
  DetailField,
  JsonOrTextBlock,
  SnLinkLoadingState,
  SnLinkNotFoundState,
} from "@features/csm-cases/components/SnLinkDetailPrimitives";

interface SmartAlertDetailModalProps {
  smartAlertId: string;
  onClose: () => void;
}

/** Maps a free-text severity value to a chip role. Unknown/absent -> "default". */
function severityRole(severity: string | undefined): SemanticRole {
  const s = severity?.toLowerCase() ?? "";
  if (s.includes("critical") || s.includes("p1") || s.includes("1")) return "error";
  if (s.includes("major") || s.includes("high") || s.includes("p2") || s.includes("2")) return "warning";
  if (s.includes("minor") || s.includes("low")) return "info";
  return "default";
}

/**
 * Read-only detail view of a single smart alert, opened from the "View smart
 * alert" marker `replaceSnLinks`/`snLinkRegistry` leaves in place of a smart-
 * alert reference embedded in a comment/work-note body. Mirrors
 * `AlertDetailModal` — a smart alert is a distinct backing record (its own
 * status/urgency/impact fields, and a `monitorUrl` out to the originating
 * monitoring tool), not just an alert with a different id, so it gets its own
 * modal rather than a shared one branching on shape.
 */
export default function SmartAlertDetailModal({
  smartAlertId,
  onClose,
}: SmartAlertDetailModalProps): JSX.Element {
  const { data, isLoading, isError } = useGetSmartAlert(smartAlertId);

  return (
    <Dialog open onClose={onClose} maxWidth="sm" fullWidth>
      <DialogTitle>
        Smart alert{data?.sourceAlertId ? ` · ${data.sourceAlertId}` : ""}
      </DialogTitle>
      <DialogContent dividers>
        {isLoading ? (
          <SnLinkLoadingState />
        ) : !data || isError ? (
          <SnLinkNotFoundState kind="smart alert" />
        ) : (
          <Box sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
            <Box sx={{ display: "flex", gap: 1, flexWrap: "wrap" }}>
              {data.severity && (
                <SemanticChip role={severityRole(data.severity)} label={data.severity} bold />
              )}
              {data.alertStatus && <SemanticChip role="default" label={data.alertStatus} />}
              {data.windowStatus && <SemanticChip role="default" label={data.windowStatus} />}
            </Box>

            {data.shortDescription && (
              <DetailField label="Summary" value={data.shortDescription} />
            )}

            <Box sx={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: 2 }}>
              <DetailField label="Source" value={data.source ?? "—"} />
              <DetailField label="Environment" value={data.environment ?? "—"} />
            </Box>

            <Box sx={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: 2 }}>
              <DetailField label="Resource" value={data.resourceName ?? "—"} />
              <DetailField label="Category" value={data.category ?? "—"} />
            </Box>

            <Box sx={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: 2 }}>
              <DetailField label="Urgency" value={data.urgency ?? "—"} />
              <DetailField label="Impact" value={data.impact ?? "—"} />
            </Box>

            <Box sx={{ display: "grid", gridTemplateColumns: "1fr 1fr 1fr", gap: 2 }}>
              <DetailField
                label="Fired"
                value={data.firedAt ? <RelativeTime iso={data.firedAt} /> : "—"}
              />
              <DetailField
                label="Received"
                value={data.receivedAt ? <RelativeTime iso={data.receivedAt} /> : "—"}
              />
              <DetailField
                label="Fire count"
                value={data.fireCount !== undefined ? String(data.fireCount) : "—"}
              />
            </Box>

            {data.details && (
              <DetailField
                label="Details"
                value={<JsonOrTextBlock value={data.details} />}
              />
            )}

            <Box sx={{ display: "flex", gap: 1, flexWrap: "wrap" }}>
              {data.monitorUrl && (
                <Button
                  size="small"
                  component="a"
                  href={data.monitorUrl}
                  target="_blank"
                  rel="noopener noreferrer"
                >
                  Open in monitoring tool
                </Button>
              )}
              {data.incidentId && (
                <Button
                  size="small"
                  component={RouterLink}
                  to={`/operations/incidents/${data.incidentId}`}
                  onClick={onClose}
                >
                  Open incident
                </Button>
              )}
            </Box>
          </Box>
        )}
      </DialogContent>
      <DialogActions>
        <Button onClick={onClose}>Close</Button>
      </DialogActions>
    </Dialog>
  );
}
