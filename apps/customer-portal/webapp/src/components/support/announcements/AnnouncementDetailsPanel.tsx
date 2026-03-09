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
  Chip,
  Paper,
  Skeleton,
  Stack,
  Typography,
  alpha,
  useTheme,
} from "@wso2/oxygen-ui";
import { ArrowLeft, Calendar, FileText } from "@wso2/oxygen-ui-icons-react";
import type { JSX, ReactElement } from "react";
import type { CaseDetails } from "@models/responses";
import {
  formatUtcToLocalNoTimezone,
  stripHtml,
  getStatusColor,
  getStatusIconElement,
  resolveColorFromTheme,
} from "@utils/support";
import ErrorIndicator from "@components/common/error-indicator/ErrorIndicator";

export interface AnnouncementDetailsPanelProps {
  data: CaseDetails | undefined;
  isLoading: boolean;
  isError: boolean;
  onBack: () => void;
}

/**
 * AnnouncementDetailsPanel displays announcement details: Back, title, date, issue type, Description.
 *
 * @param {AnnouncementDetailsPanelProps} props - Data, loading/error state, onBack.
 * @returns {JSX.Element} The rendered announcement details panel.
 */
export default function AnnouncementDetailsPanel({
  data,
  isLoading,
  isError,
  onBack,
}: AnnouncementDetailsPanelProps): JSX.Element {
  const theme = useTheme();

  if (isLoading) {
    return (
      <Box sx={{ display: "flex", flexDirection: "column", gap: 3 }}>
        <Button
          startIcon={<ArrowLeft size={16} />}
          onClick={onBack}
          sx={{ mb: 0, alignSelf: "flex-start" }}
          variant="text"
        >
          Back
        </Button>
        <Paper variant="outlined" sx={{ p: 4 }}>
          <Skeleton width="60%" height={28} sx={{ mb: 2 }} />
          <Stack direction="row" spacing={2} sx={{ mb: 2 }}>
            <Skeleton width={120} height={20} />
            <Skeleton width={80} height={20} />
          </Stack>
        </Paper>
        <Paper variant="outlined" sx={{ p: 4 }}>
          <Skeleton width="25%" height={24} sx={{ mb: 2 }} />
          <Skeleton width="100%" height={80} variant="rounded" />
        </Paper>
      </Box>
    );
  }

  if (isError || !data) {
    return (
      <Box sx={{ display: "flex", flexDirection: "column", gap: 3 }}>
        <Button
          startIcon={<ArrowLeft size={16} />}
          onClick={onBack}
          sx={{ mb: 0, alignSelf: "flex-start" }}
          variant="text"
        >
          Back
        </Button>
        <Paper variant="outlined" sx={{ p: 4, textAlign: "center" }}>
          <ErrorIndicator entityName="announcement details" size="medium" />
          <Typography variant="body2" color="text.secondary" sx={{ mt: 2 }}>
            Could not load announcement details.
          </Typography>
        </Paper>
      </Box>
    );
  }

  const statusLabel = data.status?.label;
  const statusColorPath = getStatusColor(statusLabel ?? undefined);
  const resolvedStatusColor = resolveColorFromTheme(statusColorPath, theme);
  const statusChipIcon = getStatusIconElement(statusLabel, 12);

  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
      <Button
        startIcon={<ArrowLeft size={16} />}
        onClick={onBack}
        sx={{ mb: 0, alignSelf: "flex-start" }}
        variant="text"
      >
        Back
      </Button>

      <Paper
        variant="outlined"
        elevation={0}
        sx={{
          p: 4,
          display: "flex",
          flexDirection: "column",
          gap: 1,
        }}
      >
        {data.number && (
          <Box
            sx={{
              display: "flex",
              alignItems: "center",
              gap: 1,
              mb: 0.5,
              flexWrap: "wrap",
            }}
          >
            <Typography variant="caption" color="text.secondary">
              {data.number}
            </Typography>
            {statusLabel && (
              <Chip
                size="small"
                variant="outlined"
                label={statusLabel}
                icon={statusChipIcon as ReactElement}
                sx={{
                  bgcolor: alpha(resolvedStatusColor, 0.1),
                  color: resolvedStatusColor,
                  height: 20,
                  fontSize: "0.75rem",
                  px: 0,
                  "& .MuiChip-icon": {
                    color: "inherit",
                    ml: "6px",
                    mr: "6px",
                  },
                  "& .MuiChip-label": {
                    pl: 0,
                    pr: "6px",
                  },
                }}
              />
            )}
          </Box>
        )}

        <Typography
          variant="h6"
          color="text.primary"
          sx={{ mb: 1, fontWeight: 500 }}
        >
          {data.title || "--"}
        </Typography>

        <Stack
          direction="row"
          spacing={2}
          alignItems="center"
          sx={{ flexWrap: "wrap", gap: 1 }}
        >
          <Stack direction="row" spacing={1} alignItems="center">
            <Calendar
              size={16}
              color={theme.palette.text.secondary}
              aria-hidden
            />
            <Typography variant="body2" color="text.secondary">
              {formatUtcToLocalNoTimezone(data.createdOn) || "--"}
            </Typography>
          </Stack>
          {data.issueType?.label && (
            <Stack direction="row" spacing={1} alignItems="center">
              <FileText
                size={16}
                color={theme.palette.text.secondary}
                aria-hidden
              />
              <Typography variant="body2" color="text.secondary">
                {data.issueType.label}
              </Typography>
            </Stack>
          )}
        </Stack>
      </Paper>

      <Paper
        variant="outlined"
        elevation={0}
        sx={{
          p: 4,
          display: "flex",
          flexDirection: "column",
          gap: 2,
        }}
      >
        <Typography variant="h6" color="text.primary">
          Description
        </Typography>
        <Typography variant="body2" color="text.primary">
          {data.description ? stripHtml(data.description) : "Nothing"}
        </Typography>
      </Paper>
    </Box>
  );
}
