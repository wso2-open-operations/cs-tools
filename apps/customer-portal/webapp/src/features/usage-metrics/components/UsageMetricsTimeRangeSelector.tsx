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
  Typography,
  DatePickers,
  AdapterDateFns,
} from "@wso2/oxygen-ui";
import { Calendar } from "@wso2/oxygen-ui-icons-react";
import { format } from "date-fns";
import type { JSX } from "react";
import { useControlledDatePickerValue, isPastOrPresentDate } from "@hooks/useControlledDatePickerValue";
import {
  USAGE_METRICS_CUSTOM_RANGE_APPLY,
  USAGE_METRICS_CUSTOM_RANGE_BUTTON,
  USAGE_METRICS_CUSTOM_RANGE_CANCEL,
  USAGE_METRICS_CUSTOM_RANGE_INVALID_ORDER,
  USAGE_METRICS_CUSTOM_RANGE_MAX_DAYS,
  USAGE_METRICS_CUSTOM_RANGE_PLACEHOLDER,
  USAGE_METRICS_CUSTOM_RANGE_TOO_LONG,
  USAGE_METRICS_CUSTOM_RANGE_TO,
  USAGE_METRICS_PRESET_TIME_RANGES,
  USAGE_TIME_RANGE_LABELS,
  USAGE_METRICS_TIME_RANGE_HEADING,
} from "@features/usage-metrics/constants/usageMetricsConstants";
import {
  UsageMetricsInnerTabId,
  type UsageMetricsTimeRangeSelectorProps,
} from "@features/usage-metrics/types/usageMetrics";
import { UsageTimeRange } from "@features/project-details/types/usage";
import { getUsagePresetShortLabel } from "@features/usage-metrics/utils/usageMetricsTab";

const { LocalizationProvider, DatePicker } = DatePickers;

function parseDateOnly(value: string): Date | null {
  if (!value) return null;
  const match = /^(\d{4})-(\d{2})-(\d{2})/.exec(value);
  if (!match) return null;
  const date = new Date(Number(match[1]), Number(match[2]) - 1, Number(match[3]));
  return Number.isNaN(date.getTime()) ? null : date;
}

function formatDateOnly(date: Date): string {
  return format(date, "yyyy-MM-dd");
}

/**
 * Preset and custom date range controls for Usage & Metrics panels.
 *
 * @param props - Range state and handlers.
 * @returns {JSX.Element} Toolbar row.
 */
export default function UsageMetricsTimeRangeSelector({
  innerTab,
  timeRange,
  onTimeRangeChange,
  onClearCustomApplied,
  customStart,
  customEnd,
  onCustomStartChange,
  onCustomEndChange,
  onApplyCustom,
  onCancelCustom,
  appliedCustomStart,
  appliedCustomEnd,
  rightAction,
}: UsageMetricsTimeRangeSelectorProps): JSX.Element {
  const timeLabel = USAGE_TIME_RANGE_LABELS[timeRange];

  // isComplete: isPastOrPresentDate -- both pickers below are `disableFuture`,
  // but MUI's own `disableFuture` only disables the calendar popup's future
  // days; it doesn't stop a hand-typed future date from reaching onChange.
  const customStartPicker = useControlledDatePickerValue({
    value: customStart,
    onChange: onCustomStartChange,
    parse: parseDateOnly,
    format: formatDateOnly,
    isComplete: isPastOrPresentDate,
  });
  const customEndPicker = useControlledDatePickerValue({
    value: customEnd,
    onChange: onCustomEndChange,
    parse: parseDateOnly,
    format: formatDateOnly,
    isComplete: isPastOrPresentDate,
  });

  const customRangeError = (() => {
    if (!customStart || !customEnd) return null;
    const start = new Date(customStart);
    const end = new Date(customEnd);
    if (end < start) return USAGE_METRICS_CUSTOM_RANGE_INVALID_ORDER;
    const days = (end.getTime() - start.getTime()) / (1000 * 60 * 60 * 24);
    if (days > USAGE_METRICS_CUSTOM_RANGE_MAX_DAYS) return USAGE_METRICS_CUSTOM_RANGE_TOO_LONG;
    return null;
  })();

  return (
    <Box
      sx={{
        display: "flex",
        alignItems: { xs: "flex-start", md: "center" },
        justifyContent: "space-between",
        gap: 2,
        mb: 1,
        mt: innerTab !== UsageMetricsInnerTabId.OVERVIEW ? 1 : 0,
        width: "100%",
        minWidth: 0,
        flexWrap: { xs: "wrap", md: "nowrap" },
      }}
    >
      <Box
        sx={{
          display: "flex",
          alignItems: "center",
          gap: 1.5,
          flexShrink: 1,
          minWidth: 0,
          flexWrap: "wrap",
        }}
      >
        <Calendar size={18} />
        <Typography variant="body2" sx={{ fontWeight: 600 }}>
          {USAGE_METRICS_TIME_RANGE_HEADING}
        </Typography>
        <Box sx={{ display: "flex", alignItems: "center", gap: 1, flexWrap: "wrap" }}>
          {USAGE_METRICS_PRESET_TIME_RANGES.map((preset) => {
            const selected = timeRange === preset;
            return (
              <Button
                key={preset}
                size="small"
                variant={selected ? "contained" : "outlined"}
                color={selected ? "warning" : "inherit"}
                onClick={() => {
                  onTimeRangeChange(preset);
                  onClearCustomApplied();
                }}
                sx={{ textTransform: "none", minWidth: 48 }}
              >
                {getUsagePresetShortLabel(preset)}
              </Button>
            );
          })}
          <Button
            size="small"
            variant={
              timeRange === UsageTimeRange.CUSTOM ? "contained" : "outlined"
            }
            color={timeRange === UsageTimeRange.CUSTOM ? "warning" : "inherit"}
            onClick={() => onTimeRangeChange(UsageTimeRange.CUSTOM)}
            sx={{ textTransform: "none", minWidth: 48 }}
          >
            {USAGE_METRICS_CUSTOM_RANGE_BUTTON}
          </Button>

          {timeRange === UsageTimeRange.CUSTOM && (
            <Box sx={{ display: "flex", flexDirection: "column", gap: 0.5, ml: 1 }}>
              <Box sx={{ display: "flex", alignItems: "center", gap: 2, flexWrap: "wrap" }}>
                <LocalizationProvider dateAdapter={AdapterDateFns}>
                  <Box sx={{ display: "flex", alignItems: "center", gap: 1, flexWrap: "wrap" }}>
                    <DatePicker
                      value={customStartPicker.localDate}
                      disableFuture
                      maxDate={customEndPicker.localDate ?? undefined}
                      onChange={customStartPicker.handleChange}
                      slotProps={{
                        textField: {
                          size: "small",
                          error: !!customRangeError,
                          sx: { minWidth: 160, maxWidth: "100%" },
                          slotProps: { htmlInput: { "aria-label": "Custom range start date" } },
                        },
                        field: { clearable: true, onClear: customStartPicker.handleClear },
                      }}
                    />
                    <Typography
                      variant="body2"
                      color="text.secondary"
                      sx={{ mx: 0.5 }}
                    >
                      {USAGE_METRICS_CUSTOM_RANGE_TO}
                    </Typography>
                    <DatePicker
                      value={customEndPicker.localDate}
                      disableFuture
                      minDate={customStartPicker.localDate ?? undefined}
                      onChange={customEndPicker.handleChange}
                      slotProps={{
                        textField: {
                          size: "small",
                          error: !!customRangeError,
                          sx: { minWidth: 160, maxWidth: "100%" },
                          slotProps: { htmlInput: { "aria-label": "Custom range end date" } },
                        },
                        field: { clearable: true, onClear: customEndPicker.handleClear },
                      }}
                    />
                  </Box>
                </LocalizationProvider>
                <Box sx={{ display: "flex", gap: 1 }}>
                  <Button
                    size="small"
                    variant="contained"
                    color="warning"
                    onClick={onApplyCustom}
                    disabled={!customStart || !customEnd || !!customRangeError}
                  >
                    {USAGE_METRICS_CUSTOM_RANGE_APPLY}
                  </Button>
                  <Button
                    size="small"
                    variant="outlined"
                    color="inherit"
                    onClick={onCancelCustom}
                  >
                    {USAGE_METRICS_CUSTOM_RANGE_CANCEL}
                  </Button>
                </Box>
              </Box>
              {customRangeError && (
                <Typography variant="caption" color="error">
                  {customRangeError}
                </Typography>
              )}
            </Box>
          )}
        </Box>
      </Box>

      <Box sx={{ display: "flex", alignItems: "center", gap: 1, flexShrink: 0 }}>
        <Typography
          variant="body2"
          color="text.secondary"
          sx={{
            minWidth: { xs: "auto", md: 100 },
            textAlign: { xs: "left", md: "right" },
          }}
        >
          {timeRange === UsageTimeRange.CUSTOM
            ? appliedCustomStart && appliedCustomEnd
              ? `${appliedCustomStart} to ${appliedCustomEnd}`
              : USAGE_METRICS_CUSTOM_RANGE_PLACEHOLDER
            : timeLabel}
        </Typography>
        {rightAction}
      </Box>
    </Box>
  );
}
