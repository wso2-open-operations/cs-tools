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

import {
  Box,
  Button,
  Card,
  Chip,
  IconButton,
  Skeleton,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TableRow,
  Typography,
} from "@wso2/oxygen-ui";
import { Clock, RefreshCw } from "@wso2/oxygen-ui-icons-react";
import type { JSX } from "react";
import { formatAbsoluteForUser } from "@utils/dateTime";
import { formatRelativeTime } from "@features/csm-dashboard/utils/abtDashboard";
import { useGetCsmCaseSlas } from "@features/csm-cases/api/useGetCsmCaseSlas";
import type { CaseSla, SlaStage } from "@features/csm-cases/types/csmCases";

const SLA_STAGE_COLOR: Record<
  SlaStage,
  "info" | "warning" | "success" | "default" | "error"
> = {
  in_progress: "info",
  paused: "warning",
  completed: "success",
  cancelled: "default",
  breached: "error",
};

const SLA_TABLE_COLUMNS = [
  "SLA definition",
  "Target",
  "Stage",
  "Business time left",
  "Business elapsed time",
  "Business elapsed %",
  "Start time",
  "Stop time",
];

/** Formats a backend UTC timestamp for the SLA table's start/stop columns. */
function formatSlaDateTime(iso: string | null): string {
  if (!iso) return "—";
  return formatAbsoluteForUser(iso) ?? "—";
}

/** Chip color for a row: a breached SLA is always shown in error styling. */
function stageColor(sla: CaseSla): "info" | "warning" | "success" | "default" | "error" {
  return sla.hasBreached ? "error" : (SLA_STAGE_COLOR[sla.stage] ?? "default");
}

interface CaseSlaTableProps {
  caseId: string;
  /** Whether the SLA tab is the active tab — gates the (lazy) fetch. */
  active: boolean;
}

/**
 * Table of the case's SLA records, shown on the SLA
 * tab of the case detail page. Fetches the case's SLA records lazily: the
 * query only runs once `active` is true so switching between tabs doesn't
 * fire the request until the SLA tab is actually opened.
 */
export function CaseSlaTable({ caseId, active }: CaseSlaTableProps): JSX.Element {
  const { data, isLoading, isError, isFetching, dataUpdatedAt, refetch } =
    useGetCsmCaseSlas(caseId, { enabled: active });

  const slas = data?.slas ?? [];
  const count = data?.count ?? slas.length;

  return (
    <Card sx={{ p: 2.5, display: "flex", flexDirection: "column", gap: 2 }}>
      {/* Header */}
      <Box
        sx={{
          display: "flex",
          alignItems: "center",
          justifyContent: "space-between",
          gap: 1,
          flexWrap: "wrap",
        }}
      >
        <Box sx={{ display: "flex", alignItems: "center", gap: 0.75 }}>
          <Clock size={16} />
          <Typography variant="subtitle2">SLAs</Typography>
          {!isLoading && !isError && (
            <Chip size="small" variant="outlined" label={`${count} total`} />
          )}
        </Box>
        <Box sx={{ display: "flex", alignItems: "center", gap: 1 }}>
          {dataUpdatedAt ? (
            <Typography variant="caption" color="text.secondary">
              Last refreshed {formatRelativeTime(new Date(dataUpdatedAt).toISOString())}
            </Typography>
          ) : null}
          <IconButton
            size="small"
            onClick={() => void refetch()}
            disabled={isFetching}
            aria-label="Refresh SLAs"
          >
            <RefreshCw size={14} />
          </IconButton>
        </Box>
      </Box>

      {/* Content */}
      {isError ? (
        <Box
          sx={{
            display: "flex",
            flexDirection: "column",
            alignItems: "center",
            gap: 1,
            py: 3,
          }}
        >
          <Typography variant="body2" color="error">
            Could not load SLAs for this case.
          </Typography>
          <Button
            size="small"
            variant="outlined"
            startIcon={<RefreshCw size={14} />}
            onClick={() => void refetch()}
            sx={{ textTransform: "none" }}
          >
            Retry
          </Button>
        </Box>
      ) : (
        <TableContainer>
          <Table size="small">
            <TableHead>
              <TableRow>
                {SLA_TABLE_COLUMNS.map((col) => (
                  <TableCell key={col}>{col}</TableCell>
                ))}
              </TableRow>
            </TableHead>
            <TableBody>
              {isLoading ? (
                [0, 1, 2].map((i) => (
                  <TableRow key={i}>
                    {SLA_TABLE_COLUMNS.map((col) => (
                      <TableCell key={col}>
                        <Skeleton variant="text" />
                      </TableCell>
                    ))}
                  </TableRow>
                ))
              ) : slas.length === 0 ? (
                <TableRow>
                  <TableCell
                    colSpan={SLA_TABLE_COLUMNS.length}
                    align="center"
                    sx={{ py: 4 }}
                  >
                    <Typography variant="body2" color="text.secondary">
                      No SLAs on this case.
                    </Typography>
                  </TableCell>
                </TableRow>
              ) : (
                slas.map((sla) => (
                  <TableRow key={sla.id} hover>
                    <TableCell>{sla.definition}</TableCell>
                    <TableCell>{sla.target ?? "—"}</TableCell>
                    <TableCell>
                      <Chip
                        size="small"
                        variant="outlined"
                        color={stageColor(sla)}
                        label={sla.stageLabel}
                      />
                    </TableCell>
                    <TableCell>{sla.businessTimeLeftLabel}</TableCell>
                    <TableCell>{sla.businessElapsedLabel}</TableCell>
                    <TableCell>{`${Math.round(sla.businessElapsedPercent)}%`}</TableCell>
                    <TableCell>{formatSlaDateTime(sla.startTime)}</TableCell>
                    <TableCell>{formatSlaDateTime(sla.stopTime)}</TableCell>
                  </TableRow>
                ))
              )}
            </TableBody>
          </Table>
        </TableContainer>
      )}
    </Card>
  );
}

export default CaseSlaTable;
