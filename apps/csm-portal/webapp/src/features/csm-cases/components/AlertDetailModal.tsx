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
import { useGetAlert } from "@features/csm-cases/api/useSnLinkEntities";
import {
  DetailField,
  JsonOrTextBlock,
  SnLinkLoadingState,
  SnLinkNotFoundState,
} from "@features/csm-cases/components/SnLinkDetailPrimitives";

interface AlertDetailModalProps {
  alertId: string;
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
 * Read-only detail view of a single alert, opened from the "View alert"
 * marker `replaceSnLinks`/`snLinkRegistry` leaves in place of an alert
 * reference embedded in a comment/work-note body.
 */
export default function AlertDetailModal({
  alertId,
  onClose,
}: AlertDetailModalProps): JSX.Element {
  const { data, isLoading, isError } = useGetAlert(alertId);

  return (
    <Dialog open onClose={onClose} maxWidth="sm" fullWidth>
      <DialogTitle>Alert{data?.number ? ` · ${data.number}` : ""}</DialogTitle>
      <DialogContent dividers>
        {isLoading ? (
          <SnLinkLoadingState />
        ) : !data || isError ? (
          <SnLinkNotFoundState kind="alert" />
        ) : (
          <Box sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
            <Box sx={{ display: "flex", gap: 1, flexWrap: "wrap" }}>
              {data.severity && (
                <SemanticChip role={severityRole(data.severity)} label={data.severity} bold />
              )}
              {data.category && <SemanticChip role="default" label={data.category} />}
            </Box>

            <Box sx={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: 2 }}>
              <DetailField label="Source" value={data.source ?? "—"} />
              <DetailField label="Environment" value={data.environment ?? "—"} />
            </Box>

            <Box sx={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: 2 }}>
              <DetailField label="Metric" value={data.metricName ?? "—"} />
              <DetailField
                label="Created"
                value={
                  data.createdOn ? <RelativeTime iso={data.createdOn} /> : "—"
                }
              />
            </Box>

            {data.description && (
              <DetailField
                label="Description"
                value={<JsonOrTextBlock value={data.description} />}
              />
            )}

            {data.incidentId && (
              <DetailField
                label="Linked incident"
                value={
                  <Button
                    size="small"
                    component={RouterLink}
                    to={`/operations/incidents/${data.incidentId}`}
                    onClick={onClose}
                  >
                    Open incident
                  </Button>
                }
              />
            )}
          </Box>
        )}
      </DialogContent>
      <DialogActions>
        <Button onClick={onClose}>Close</Button>
      </DialogActions>
    </Dialog>
  );
}
