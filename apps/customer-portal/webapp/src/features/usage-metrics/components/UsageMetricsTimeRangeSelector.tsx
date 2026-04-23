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
  InputAdornment,
  TextField,
  Typography,
} from "@wso2/oxygen-ui";
import { Calendar } from "@wso2/oxygen-ui-icons-react";
import type { JSX } from "react";
import {
  USAGE_METRICS_CUSTOM_RANGE_APPLY,
  USAGE_METRICS_CUSTOM_RANGE_BUTTON,
  USAGE_METRICS_CUSTOM_RANGE_CANCEL,
  USAGE_METRICS_CUSTOM_RANGE_PLACEHOLDER,
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
}: UsageMetricsTimeRangeSelectorProps): JSX.Element {
  const timeLabel = USAGE_TIME_RANGE_LABELS[timeRange];

  return (
    <Box
      sx={{
        display: "flex",
        alignItems: "center",
        justifyContent: "space-between",
        gap: 2,
        mb: 1,
        mt: innerTab !== UsageMetricsInnerTabId.OVERVIEW ? 1 : 0,
        overflowX: "auto",
        pb: 0.5,
      }}
    >
      <Box
        sx={{ display: "flex", alignItems: "center", gap: 1.5, flexShrink: 0 }}
      >
        <Calendar size={18} />
        <Typography variant="body2" sx={{ fontWeight: 600 }}>
          {USAGE_METRICS_TIME_RANGE_HEADING}
        </Typography>
        <Box sx={{ display: "flex", alignItems: "center", gap: 1 }}>
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
            <Box sx={{ display: "flex", alignItems: "center", gap: 2, ml: 1 }}>
              <Box sx={{ display: "flex", alignItems: "center", gap: 1 }}>
                <TextField
                  type="date"
                  size="small"
                  value={customStart}
                  onChange={(e) => onCustomStartChange(e.target.value)}
                  sx={{ minWidth: 160 }}
                  InputProps={{
                    startAdornment: (
                      <InputAdornment position="start">
                        <Calendar size={16} />
                      </InputAdornment>
                    ),
                  }}
                />
                <Typography
                  variant="body2"
                  color="text.secondary"
                  sx={{ mx: 0.5 }}
                >
                  {USAGE_METRICS_CUSTOM_RANGE_TO}
                </Typography>
                <TextField
                  type="date"
                  size="small"
                  value={customEnd}
                  onChange={(e) => onCustomEndChange(e.target.value)}
                  sx={{ minWidth: 160 }}
                  InputProps={{
                    startAdornment: (
                      <InputAdornment position="start">
                        <Calendar size={16} />
                      </InputAdornment>
                    ),
                  }}
                />
              </Box>
              <Box sx={{ display: "flex", gap: 1 }}>
                <Button
                  size="small"
                  variant="contained"
                  color="warning"
                  onClick={onApplyCustom}
                  disabled={!customStart || !customEnd}
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
          )}
        </Box>
      </Box>

      <Typography
        variant="body2"
        color="text.secondary"
        sx={{ minWidth: 100, textAlign: "right", flexShrink: 0 }}
      >
        {timeRange === UsageTimeRange.CUSTOM
          ? appliedCustomStart && appliedCustomEnd
            ? `${appliedCustomStart} to ${appliedCustomEnd}`
            : USAGE_METRICS_CUSTOM_RANGE_PLACEHOLDER
          : timeLabel}
      </Typography>
    </Box>
  );
}
