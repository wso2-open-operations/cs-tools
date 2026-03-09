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

import { type JSX, useState, useMemo } from "react";
import {
  Box,
  Paper,
  Typography,
  Button,
  TextField,
  InputAdornment,
  Stack,
  Divider,
  IconButton,
  colors,
} from "@wso2/oxygen-ui";
import {
  Calendar,
  Clock,
  PencilLine,
} from "@wso2/oxygen-ui-icons-react";
import { usePatchChangeRequest } from "@api/usePatchChangeRequest";
import { useErrorBanner } from "@context/error-banner/ErrorBannerContext";
import { useSuccessBanner } from "@context/success-banner/SuccessBannerContext";
import type { ChangeRequestDetails } from "@models/responses";

export interface ScheduledMaintenanceWindowCardProps {
  changeRequest: ChangeRequestDetails;
}

/** Returns true if date string is empty or invalid. */
function isDateAvailable(dateStr: string | null | undefined): boolean {
  if (!dateStr?.trim()) return false;
  const d = new Date(dateStr.replace(" ", "T"));
  return !Number.isNaN(d.getTime());
}

/**
 * Format API date string to display format (e.g. "Tuesday, December 15, 2099 at 10:00 PM").
 * Returns "Not available" for empty or invalid dates.
 *
 * @param {string} dateStr - API date string (e.g. "2026-02-28 15:30:50").
 * @returns {string} Formatted date string or "Not available".
 */
function formatDisplayDate(dateStr: string | null | undefined): string {
  if (!isDateAvailable(dateStr)) return "Not available";
  try {
    const d = new Date((dateStr ?? "").replace(" ", "T"));
    return d.toLocaleString(undefined, {
      weekday: "long",
      year: "numeric",
      month: "long",
      day: "numeric",
      hour: "numeric",
      minute: "2-digit",
    });
  } catch {
    return "Not available";
  }
}

/**
 * Format minutes as "X hours Y minutes".
 *
 * @param {number} minutes - Total minutes.
 * @returns {string} Formatted duration string.
 */
function formatDuration(minutes: number): string {
  const hours = Math.floor(minutes / 60);
  const mins = minutes % 60;
  const parts: string[] = [];
  if (hours > 0) parts.push(`${hours} hour${hours === 1 ? "" : "s"}`);
  parts.push(`${mins} minute${mins === 1 ? "" : "s"}`);
  return parts.join(" ");
}

/**
 * Convert datetime-local value to API format "YYYY-MM-DD HH:mm:ss".
 *
 * @param {string} datetimeLocal - Value from input type="datetime-local".
 * @returns {string} API format string.
 */
function toApiDatetime(datetimeLocal: string): string {
  if (!datetimeLocal) return "";
  const d = new Date(datetimeLocal);
  const y = d.getFullYear();
  const m = String(d.getMonth() + 1).padStart(2, "0");
  const day = String(d.getDate()).padStart(2, "0");
  const h = String(d.getHours()).padStart(2, "0");
  const min = String(d.getMinutes()).padStart(2, "0");
  const s = String(d.getSeconds()).padStart(2, "0");
  return `${y}-${m}-${day} ${h}:${min}:${s}`;
}

/**
 * Convert API datetime to datetime-local input value.
 *
 * @param {string} apiDatetime - API format "YYYY-MM-DD HH:mm:ss".
 * @returns {string} Value for input type="datetime-local".
 */
function toDatetimeLocal(apiDatetime: string): string {
  try {
    const d = new Date(apiDatetime.replace(" ", "T"));
    const y = d.getFullYear();
    const m = String(d.getMonth() + 1).padStart(2, "0");
    const day = String(d.getDate()).padStart(2, "0");
    const h = String(d.getHours()).padStart(2, "0");
    const min = String(d.getMinutes()).padStart(2, "0");
    return `${y}-${m}-${day}T${h}:${min}`;
  } catch {
    return "";
  }
}

/**
 * Card displaying scheduled maintenance window (Planned Start, Planned End, Duration)
 * with inline edit for planned start date.
 *
 * @param {ScheduledMaintenanceWindowCardProps} props - Change request details.
 * @returns {JSX.Element} The rendered card.
 */
export default function ScheduledMaintenanceWindowCard({
  changeRequest,
}: ScheduledMaintenanceWindowCardProps): JSX.Element {
  const { showError } = useErrorBanner();
  const { showSuccess } = useSuccessBanner();
  const patchMutation = usePatchChangeRequest(changeRequest.id);
  const [isEditing, setIsEditing] = useState(false);
  const [editValue, setEditValue] = useState(() =>
    toDatetimeLocal(changeRequest.startDate),
  );

  const durationText = useMemo(() => {
    const duration = (changeRequest as { duration?: string | number | null })
      .duration;
    if (duration == null) return "Not available";
    const mins =
      typeof duration === "number"
        ? duration
        : parseInt(String(duration), 10);
    return Number.isNaN(mins) ? "Not available" : formatDuration(mins);
  }, [changeRequest]);

  const handleUpdate = () => {
    const apiValue = toApiDatetime(editValue);
    if (!apiValue) {
      showError("Please select a valid date and time");
      return;
    }
    patchMutation.mutate(
      { plannedStartOn: apiValue },
      {
        onSuccess: () => {
          setIsEditing(false);
          showSuccess("Planned start updated successfully");
        },
        onError: (err) => {
          showError(err?.message ?? "Failed to update planned start");
        },
      },
    );
  };

  const handleCancel = () => {
    setIsEditing(false);
    setEditValue(toDatetimeLocal(changeRequest.startDate));
  };

  return (
    <Paper
      variant="outlined"
      sx={{
        display: "flex",
        flexDirection: "column",
        gap: 2,
      }}
    >
      <Box sx={{ px: 3, pt: 3 }}>
        <Box sx={{ display: "flex", alignItems: "center", gap: 1, mb: 0.5 }}>
          <Calendar size={20} color={colors.grey[600]} aria-hidden />
          <Typography variant="h6" color="text.primary">
            Scheduled Maintenance Window
          </Typography>
        </Box>
      </Box>

      <Box sx={{ px: 3, pb: 3 }}>
        <Box
          sx={{
            display: "grid",
            gridTemplateColumns: { xs: "1fr", sm: "1fr 1fr" },
            gap: 3,
          }}
        >
          <Box>
            <Typography variant="body2" color="text.secondary" sx={{ mb: 1 }}>
              Planned Start
            </Typography>
            {isEditing ? (
              <Box sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
                <TextField
                  type="datetime-local"
                  size="small"
                  value={editValue}
                  onChange={(e) => setEditValue(e.target.value)}
                  fullWidth
                  inputProps={{
                    min: new Date().toISOString().slice(0, 16),
                  }}
                  InputProps={{
                    startAdornment: (
                      <InputAdornment position="start">
                        <Clock size={16} color={colors.grey[400]} />
                      </InputAdornment>
                    ),
                  }}
                />
                <Stack direction="row" spacing={1}>
                  <Button
                    variant="contained"
                    size="small"
                    onClick={handleUpdate}
                    disabled={patchMutation.isPending}
                  >
                    Update
                  </Button>
                  <Button
                    variant="outlined"
                    size="small"
                    onClick={handleCancel}
                    disabled={patchMutation.isPending}
                  >
                    Cancel
                  </Button>
                </Stack>
              </Box>
            ) : (
              <Box sx={{ display: "flex", alignItems: "center", gap: 1 }}>
                <Clock size={16} color={colors.grey[400]} aria-hidden />
                <Typography variant="body2" color="text.primary">
                  {formatDisplayDate(changeRequest.startDate)}
                </Typography>
                {isDateAvailable(changeRequest.startDate) && (
                  <IconButton
                    size="small"
                    aria-label="Edit planned start"
                    color="primary"
                    onClick={() => {
                      setEditValue(toDatetimeLocal(changeRequest.startDate));
                      setIsEditing(true);
                    }}
                  >
                    <PencilLine size={16} />
                  </IconButton>
                )}
              </Box>
            )}
          </Box>

          <Box>
            <Typography variant="body2" color="text.secondary" sx={{ mb: 1 }}>
              Planned End
            </Typography>
            <Box sx={{ display: "flex", alignItems: "center", gap: 1 }}>
              <Clock size={16} color={colors.grey[400]} aria-hidden />
              <Typography variant="body2" color="text.primary">
                {formatDisplayDate(changeRequest.endDate)}
              </Typography>
            </Box>
          </Box>
        </Box>

        <Divider sx={{ my: 2 }} />

        <Box>
          <Typography variant="body2" color="text.secondary" sx={{ mb: 1 }}>
            Duration
          </Typography>
          <Typography variant="body2" color="text.primary">
            {durationText}
          </Typography>
        </Box>
      </Box>
    </Paper>
  );
}
