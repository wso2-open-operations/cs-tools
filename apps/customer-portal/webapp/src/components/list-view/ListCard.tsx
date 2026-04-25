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
  alpha,
  Box,
  Chip,
  Form,
  Stack,
  Typography,
  useTheme,
} from "@wso2/oxygen-ui";
import { Calendar, FileText, User } from "@wso2/oxygen-ui-icons-react";
import type { JSX, KeyboardEvent } from "react";
import type { CaseListItem } from "@features/support/types/cases";
import { getSeverityLegendColor } from "@features/dashboard/utils/dashboard";
import CaseCardDescriptionClamp from "@components/list-view/CaseCardDescriptionClamp";
import {
  formatDateTime,
  getAssignedEngineerLabel,
  getStatusColor,
  hasSeverityLabelForChip,
  mapSeverityToDisplay,
  resolveColorFromTheme,
} from "@features/support/utils/support";

export interface ListCardProps {
  caseItem: CaseListItem;
  onClick?: (caseItem: CaseListItem) => void;
  hideSeverity?: boolean;
  showInternalId?: boolean;
}

/**
 * ListCard renders a single case as a clickable card with severity, status,
 * title, description, and metadata footer.
 *
 * @param {ListCardProps} props - Case data and optional click handler.
 * @returns {JSX.Element} The rendered case card.
 */
export default function ListCard({
  caseItem,
  onClick,
  hideSeverity = false,
  showInternalId = false,
}: ListCardProps): JSX.Element {
  const theme = useTheme();
  const colorPath = getStatusColor(caseItem.status?.label);
  const resolvedColor = resolveColorFromTheme(colorPath, theme);

  return (
    <Form.CardButton
      onClick={() => onClick?.(caseItem)}
      tabIndex={onClick ? 0 : undefined}
      role={onClick ? "button" : undefined}
      onKeyDown={(event: KeyboardEvent<HTMLElement>) => {
        if (!onClick) return;
        if (
          event.key === "Enter" ||
          event.key === " " ||
          event.key === "Spacebar"
        ) {
          event.preventDefault();
          onClick(caseItem);
        }
      }}
      sx={{
        p: 3,
        display: "flex",
        flexDirection: "column",
        alignItems: "stretch",
        gap: 1,
        width: "100%",
        minWidth: 0,
      }}
    >
      <Form.CardHeader
        sx={{ p: 0 }}
        title={
          <Stack
            direction="row"
            spacing={1.5}
            alignItems="center"
            sx={{ mb: 1, flexWrap: "wrap" }}
          >
            {showInternalId && caseItem.internalId && (
              <>
                <Typography variant="body2" fontWeight={500} color="text.secondary">
                  {caseItem.internalId}
                </Typography>
                <Typography variant="body2" color="text.disabled">|</Typography>
              </>
            )}
            <Typography variant="body2" fontWeight={500} color="text.primary">
              {caseItem.number || "--"}
            </Typography>
            {!hideSeverity && hasSeverityLabelForChip(caseItem.severity?.label) && (
              <Chip
                label={mapSeverityToDisplay(caseItem.severity?.label)}
                size="small"
                variant="outlined"
                sx={{
                  bgcolor: alpha(
                    getSeverityLegendColor(caseItem.severity?.label),
                    0.1,
                  ),
                  color: getSeverityLegendColor(caseItem.severity?.label),
                  borderColor: alpha(
                    getSeverityLegendColor(caseItem.severity?.label),
                    0.3,
                  ),
                  fontWeight: 500,
                  px: 0,
                  height: 20,
                  fontSize: "0.75rem",
                  "& .MuiChip-label": { pl: "6px", pr: "6px" },
                }}
              />
            )}
            <Stack direction="row" spacing={0.75} alignItems="center">
              <Box
                sx={{
                  width: 8,
                  height: 8,
                  borderRadius: "50%",
                  bgcolor: resolvedColor,
                  flexShrink: 0,
                }}
              />
              <Typography variant="caption" color="text.secondary">
                {caseItem.status?.label || "--"}
              </Typography>
            </Stack>
            {caseItem.issueType?.label && (
              <Chip
                size="small"
                label={caseItem.issueType.label}
                variant="outlined"
                sx={{ height: 20, fontSize: "0.75rem" }}
              />
            )}
          </Stack>
        }
      />

      <Form.CardContent sx={{ p: 0 }}>
        <Typography
          variant="h6"
          color="text.primary"
          sx={{
            mb: 1,
            fontWeight: 500,
            overflowWrap: "anywhere",
            wordBreak: "break-word",
            minWidth: 0,
          }}
        >
          {caseItem.title || "--"}
        </Typography>
        <CaseCardDescriptionClamp description={caseItem.description} />
      </Form.CardContent>

      <Form.CardActions
        sx={{
          p: 0,
          justifyContent: "flex-start",
          flexWrap: "wrap",
          gap: 2,
        }}
      >
        <Box
          sx={{
            display: "flex",
            alignItems: "center",
            gap: 2,
            flexWrap: "wrap",
            minWidth: 0,
          }}
        >
          <Box
            sx={{
              display: "flex",
              alignItems: "center",
              gap: 0.5,
              minWidth: 0,
            }}
          >
            <Calendar size={14} />
            <Typography
              variant="caption"
              color="text.secondary"
              sx={{ lineHeight: 1, overflowWrap: "anywhere" }}
            >
              Created {formatDateTime(caseItem.createdOn) || "--"}
            </Typography>
          </Box>
          {caseItem.createdBy && (
            <Box
              sx={{
                display: "flex",
                alignItems: "center",
                gap: 0.5,
                minWidth: 0,
              }}
            >
              <User size={14} />
              <Typography
                variant="caption"
                color="text.secondary"
                sx={{ lineHeight: 1, overflowWrap: "anywhere" }}
              >
                Created by {caseItem.createdBy}
              </Typography>
            </Box>
          )}
          {(() => {
            const assignedLabel = getAssignedEngineerLabel(
              caseItem.assignedEngineer,
            );
            return assignedLabel ? (
              <Box
                sx={{
                  display: "flex",
                  alignItems: "center",
                  gap: 0.5,
                  minWidth: 0,
                }}
              >
                <User size={14} />
                <Typography
                  variant="caption"
                  color="text.secondary"
                  sx={{ lineHeight: 1, overflowWrap: "anywhere" }}
                >
                  Assigned to {assignedLabel}
                </Typography>
              </Box>
            ) : null;
          })()}
          {caseItem.deployment?.label && (
            <Box
              sx={{
                display: "flex",
                alignItems: "center",
                gap: 0.5,
                minWidth: 0,
              }}
            >
              <FileText size={14} />
              <Typography
                variant="caption"
                color="text.secondary"
              sx={{ lineHeight: 1, overflowWrap: "anywhere" }}
              >
                {caseItem.deployment.label}
              </Typography>
            </Box>
          )}
        </Box>
      </Form.CardActions>
    </Form.CardButton>
  );
}
