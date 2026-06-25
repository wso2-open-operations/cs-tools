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
  Chip,
  CircularProgress,
  Divider,
  Stack,
  Typography,
  alpha,
  useTheme,
} from "@wso2/oxygen-ui";
import { ArrowRight, Flag, TriangleAlert } from "@wso2/oxygen-ui-icons-react";
import { type JSX } from "react";
import { usePostCaseEscalationsSearch } from "@features/support/api/usePostCaseEscalationsSearch";
import useGetUserDetails from "@features/settings/api/useGetUserDetails";
import type { EscalationRecord } from "@features/support/types/cases";

type Props = {
  caseId: string;
  caseCreatedOn?: string | null;
};

function formatDate(iso?: string | null): string {
  if (!iso) return "—";
  return new Date(iso).toLocaleString(undefined, {
    year: "numeric",
    month: "short",
    day: "numeric",
    hour: "2-digit",
    minute: "2-digit",
  });
}

function EscalationTimelineItem({
  record,
  currentUserEmail,
  isLast,
}: {
  record: EscalationRecord;
  currentUserEmail: string;
  isLast: boolean;
}): JSX.Element {
  const theme = useTheme();
  const amberColor = theme.palette.warning.main;
  const amberLight = theme.palette.warning.light;

  const isCurrentUser =
    !!currentUserEmail &&
    record.createdBy.toLowerCase() === currentUserEmail.toLowerCase();
  const actor = isCurrentUser ? "You" : record.createdBy;

  return (
    <Box sx={{ display: "flex", gap: 2 }}>
      {/* Left: icon + line */}
      <Box sx={{ display: "flex", flexDirection: "column", alignItems: "center", width: 32, flexShrink: 0 }}>
        <Box
          sx={{
            width: 32,
            height: 32,
            borderRadius: "50%",
            bgcolor: alpha(amberLight, 0.2),
            border: `2px solid ${alpha(amberColor, 0.4)}`,
            display: "flex",
            alignItems: "center",
            justifyContent: "center",
            flexShrink: 0,
          }}
        >
          <TriangleAlert size={15} color={amberColor} />
        </Box>
        {!isLast && (
          <Box
            sx={{
              width: 2,
              flex: 1,
              mt: 0.5,
              bgcolor: "divider",
              minHeight: 24,
            }}
          />
        )}
      </Box>

      {/* Right: content */}
      <Box sx={{ flex: 1, pb: isLast ? 0 : 3 }}>
        {/* Level row */}
        <Box sx={{ display: "flex", alignItems: "center", gap: 1, flexWrap: "wrap", mb: 0.75 }}>
          <Chip
            label={record.previousLevel?.label ?? "—"}
            size="small"
            variant="outlined"
            sx={{ fontWeight: 600, fontSize: "0.7rem", height: 20 }}
          />
          <ArrowRight size={14} color={theme.palette.text.secondary} />
          <Chip
            label={record.currentLevel?.label ?? "—"}
            size="small"
            variant="outlined"
            sx={{
              fontWeight: 600,
              fontSize: "0.7rem",
              height: 20,
              color: amberColor,
              borderColor: alpha(amberColor, 0.5),
              bgcolor: alpha(amberLight, 0.12),
            }}
          />
          <Typography variant="caption" color="text.secondary">
            by{" "}
            <Box component="span" fontWeight={500} color="text.primary">
              {actor}
            </Box>
          </Typography>
        </Box>

        {/* Timestamp */}
        <Typography variant="caption" color="text.secondary" sx={{ display: "block", mb: 0.75 }}>
          {formatDate(record.createdOn)}
        </Typography>

        {/* Notified */}
        {record.notificationSentTo && record.notificationSentTo.length > 0 && (
          <Typography variant="caption" color="text.secondary" sx={{ display: "block", mb: 0.75 }}>
            Notified: {record.notificationSentTo.map((u) => u.name ?? u.userName).join(", ")}
          </Typography>
        )}

        {/* Reason */}
        {record.reason && (
          <Box
            sx={{
              bgcolor: "action.hover",
              borderRadius: 1,
              px: 1.5,
              py: 1,
              mt: 0.5,
            }}
          >
            <Typography variant="caption" color="text.secondary">
              {record.reason}
            </Typography>
          </Box>
        )}
      </Box>
    </Box>
  );
}

/**
 * Escalation history timeline panel for a case.
 * Shows all escalation records newest-first, with the initial EL0 state pinned at the bottom.
 *
 * @param {Props} props - caseId and caseCreatedOn.
 * @returns {JSX.Element} The escalation history panel.
 */
export default function CaseEscalationHistoryPanel({ caseId, caseCreatedOn }: Props): JSX.Element {
  const theme = useTheme();
  const { data: userDetails } = useGetUserDetails();
  const currentUserEmail = userDetails?.email?.toLowerCase() ?? "";

  const { data, isLoading, isError } = usePostCaseEscalationsSearch(caseId);

  if (isLoading) {
    return (
      <Box sx={{ display: "flex", justifyContent: "center", py: 6 }}>
        <CircularProgress size={28} />
      </Box>
    );
  }

  if (isError) {
    return (
      <Typography variant="body2" color="error" sx={{ py: 4, textAlign: "center" }}>
        Failed to load escalation history.
      </Typography>
    );
  }

  const records = data?.escalations ?? [];

  return (
    <Box>
      <Stack direction="row" spacing={1} alignItems="center" sx={{ mb: 2 }}>
        <TriangleAlert size={16} color={theme.palette.warning.main} />
        <Typography variant="subtitle2" fontWeight={600}>
          Escalation History
        </Typography>
        {records.length > 0 && (
          <Chip
            label={records.length}
            size="small"
            sx={{
              height: 18,
              fontSize: "0.7rem",
              bgcolor: alpha(theme.palette.warning.light, 0.2),
              color: theme.palette.warning.dark,
            }}
          />
        )}
      </Stack>

      {records.length === 0 ? (
        <Box
          sx={{
            display: "flex",
            flexDirection: "column",
            alignItems: "center",
            py: 6,
            gap: 1,
          }}
        >
          <Flag size={32} color={theme.palette.text.disabled} />
          <Typography variant="body2" color="text.secondary">
            No escalations recorded for this case.
          </Typography>
        </Box>
      ) : (
        <Box>
          {records.map((record, idx) => (
            <EscalationTimelineItem
              key={record.id}
              record={record}
              currentUserEmail={currentUserEmail}
              isLast={false}
            />
          ))}

          {/* Initial state anchor */}
          <Box sx={{ display: "flex", gap: 2 }}>
            <Box sx={{ display: "flex", flexDirection: "column", alignItems: "center", width: 32, flexShrink: 0 }}>
              <Box
                sx={{
                  width: 32,
                  height: 32,
                  borderRadius: "50%",
                  bgcolor: "action.hover",
                  border: `2px solid`,
                  borderColor: "divider",
                  display: "flex",
                  alignItems: "center",
                  justifyContent: "center",
                  flexShrink: 0,
                }}
              >
                <Flag size={14} color={theme.palette.text.secondary} />
              </Box>
            </Box>
            <Box sx={{ flex: 1, pb: 0 }}>
              <Box sx={{ display: "flex", alignItems: "center", gap: 1, mb: 0.5, mt: 0.5 }}>
                <Chip
                  label="EL0"
                  size="small"
                  variant="outlined"
                  sx={{ fontWeight: 600, fontSize: "0.7rem", height: 20 }}
                />
                <Typography variant="caption" color="text.secondary">
                  Case created
                </Typography>
              </Box>
              {caseCreatedOn && (
                <Typography variant="caption" color="text.secondary">
                  {formatDate(caseCreatedOn)}
                </Typography>
              )}
            </Box>
          </Box>

          <Divider sx={{ mt: 2 }} />
        </Box>
      )}
    </Box>
  );
}
