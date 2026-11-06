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

import { alpha, Box, Button, Chip, Form, Stack, Typography, useTheme } from "@wso2/oxygen-ui";
import {
  Calendar,
  CircleCheck,
  FileText,
  History,
  RotateCcw,
  User,
} from "@wso2/oxygen-ui-icons-react";
import type { JSX } from "react";
import type { PendingVerificationView } from "@features/support/types/pendingVerification";
import { getSeverityDisplay, RECORD_TYPE_ICONS } from "@features/support/utils/pendingVerification";
import { formatRelativeTime } from "@features/support/utils/support";

export interface PendingVerificationCardProps {
  record: PendingVerificationView;
  onViewCase?: (record: PendingVerificationView) => void;
  onMarkVerified?: (record: PendingVerificationView) => void;
}

// Matches the "<CS-number> | <internal-id>" pairing the Resolved Cases list
// already shows (formatCasesTableCaseIdentifier in casesTable.ts), minus that
// helper's own "ID: " prefix -- this card has no such label context.
function formatPendingVerificationIdentifier(caseId: string, internalId?: string): string {
  const number = caseId.trim();
  const wso2Id = internalId?.trim() ?? "";
  if (number && wso2Id) return `${number} | ${wso2Id}`;
  return number || wso2Id || "--";
}

/**
 * PendingVerificationCard renders a single pending-verification record —
 * identifier, severity, added-reason, title, note, and footer metadata.
 * The whole card navigates to the record (onViewCase) on click; "Mark
 * Verified" is a separate action that stops propagation so it doesn't
 * also trigger navigation.
 *
 * @param {PendingVerificationCardProps} props - Record data and action callbacks.
 * @returns {JSX.Element} The rendered card.
 */
export default function PendingVerificationCard({
  record,
  onViewCase,
  onMarkVerified,
}: PendingVerificationCardProps): JSX.Element {
  const theme = useTheme();
  const isVerified = !!record.verifiedAt;
  const severity = getSeverityDisplay(record.severity);
  const isAutoClosed = record.addedReason === "auto-closed";
  const TypeIcon = RECORD_TYPE_ICONS[record.recordType];

  return (
    <Form.CardButton
      onClick={() => onViewCase?.(record)}
      sx={{
        p: 3,
        display: "flex",
        flexDirection: "row",
        alignItems: "flex-start",
        gap: 2,
        width: "100%",
        minWidth: 0,
      }}
    >
      <Box sx={{ flex: 1, minWidth: 0 }}>
        <Stack direction="row" spacing={1.5} alignItems="center" sx={{ mb: 1, flexWrap: "wrap" }}>
          <Box sx={{ display: "flex", alignItems: "center", gap: 0.75 }}>
            <TypeIcon size={14} color={theme.palette.primary.main} />
            <Typography variant="body2" fontWeight={600} color="text.primary">
              {formatPendingVerificationIdentifier(record.caseId, record.internalId)}
            </Typography>
          </Box>
          {severity && (
            <Chip
              size="small"
              label={severity.label}
              variant="outlined"
              sx={{
                height: 20,
                fontSize: "0.75rem",
                color: severity.color,
                borderColor: alpha(severity.color, 0.4),
                bgcolor: alpha(severity.color, 0.08),
              }}
            />
          )}
          <Chip
            size="small"
            icon={isAutoClosed ? <RotateCcw size={11} /> : <FileText size={11} />}
            label={isAutoClosed ? "Auto-closed" : "Manually Added"}
            variant="outlined"
            sx={{ height: 20, fontSize: "0.75rem" }}
          />
          {isAutoClosed && record.previousStatus && (
            <Chip
              size="small"
              icon={<History size={11} />}
              label={`From ${record.previousStatus}`}
              variant="outlined"
              sx={{ height: 20, fontSize: "0.75rem", color: "text.secondary" }}
            />
          )}
          {isVerified && (
            <Chip
              icon={<CircleCheck size={11} />}
              label="Verified"
              size="small"
              variant="outlined"
              sx={{
                color: theme.palette.success.dark,
                borderColor: alpha(theme.palette.success.main, 0.5),
                bgcolor: alpha(theme.palette.success.light, 0.12),
                fontWeight: 500,
                height: 20,
                fontSize: "0.7rem",
                "& .MuiChip-icon": { color: theme.palette.success.main, ml: "4px", mr: "2px" },
                "& .MuiChip-label": { pl: "4px", pr: "8px" },
              }}
            />
          )}
        </Stack>

        <Typography
          variant="h6"
          color="text.primary"
          sx={{ mb: 1, fontWeight: 500, overflowWrap: "anywhere", wordBreak: "break-word", minWidth: 0 }}
        >
          {record.caseTitle || "--"}
        </Typography>
        {record.note && (
          <Typography
            variant="body2"
            color="text.secondary"
            sx={{
              mb: 1,
              overflow: "hidden",
              textOverflow: "ellipsis",
              display: "-webkit-box",
              WebkitLineClamp: 2,
              WebkitBoxOrient: "vertical",
            }}
          >
            {record.note}
          </Typography>
        )}

        <Box sx={{ display: "flex", alignItems: "center", gap: 2, flexWrap: "wrap", minWidth: 0 }}>
          <Box sx={{ display: "flex", alignItems: "center", gap: 0.5, minWidth: 0 }}>
            <Calendar size={14} />
            <Typography variant="caption" color="text.secondary" sx={{ lineHeight: 1, overflowWrap: "anywhere" }}>
              {isVerified ? "Verified" : "Added"}{" "}
              {formatRelativeTime(isVerified ? (record.verifiedAt ?? undefined) : record.addedAt)}
            </Typography>
          </Box>
          <Box sx={{ display: "flex", alignItems: "center", gap: 0.5, minWidth: 0 }}>
            <User size={14} />
            <Typography variant="caption" color="text.secondary" sx={{ lineHeight: 1, overflowWrap: "anywhere" }}>
              {isVerified ? "Verified by" : "Added by"}{" "}
              {isVerified ? (record.verifiedBy ?? record.addedBy) : record.addedBy}
            </Typography>
          </Box>
        </Box>
      </Box>

      {!isVerified && (
        <Stack spacing={1} sx={{ flexShrink: 0, alignItems: "stretch" }}>
          <Button
            variant="contained"
            color="success"
            size="small"
            startIcon={<CircleCheck size={12} />}
            onClick={(e) => {
              e.stopPropagation();
              onMarkVerified?.(record);
            }}
            sx={{ fontSize: "0.7rem", whiteSpace: "nowrap" }}
          >
            Mark Verified
          </Button>
        </Stack>
      )}
    </Form.CardButton>
  );
}
