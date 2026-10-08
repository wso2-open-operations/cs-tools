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
import { Box, Typography } from "@wso2/oxygen-ui";
import { LocalizationProvider } from "@mui/x-date-pickers/LocalizationProvider";
import { AdapterDateFns } from "@mui/x-date-pickers/AdapterDateFns";
import { DatePicker } from "@mui/x-date-pickers/DatePicker";
import { useControlledDatePickerValue, isPastOrPresentDate } from "@hooks/useControlledDatePickerValue";
import {
  toUtcStartOfDay,
  toUtcEndOfDay,
} from "@features/support/utils/support";

function parseUtcIso(value: string | undefined): Date | null {
  if (!value) return null;
  // Extract YYYY-MM-DD from the UTC string and build a local-midnight Date so the
  // picker displays the same calendar day the user selected, regardless of timezone.
  const match = /^(\d{4})-(\d{2})-(\d{2})/.exec(value);
  if (!match) return null;
  const date = new Date(Number(match[1]), Number(match[2]) - 1, Number(match[3]));
  return Number.isNaN(date.getTime()) ? null : date;
}

function parseUtcIsoEndDate(value: string | undefined): Date | null {
  if (!value) return null;
  // The stored end value is start-of-next-day (exclusive upper bound).
  // Subtract 1 day to recover the actual calendar day the user selected.
  const match = /^(\d{4})-(\d{2})-(\d{2})/.exec(value);
  if (!match) return null;
  const date = new Date(Number(match[1]), Number(match[2]) - 1, Number(match[3]) - 1);
  return Number.isNaN(date.getTime()) ? null : date;
}

// MUI's DatePicker has keyboard-editable year/month/day sections, so onChange
// can fire mid-typing with a technically-valid, non-NaN Date whose year is
// still incomplete (e.g. the user typed "2" and tabbed away before finishing
// "2026") -- `!isNaN(date.getTime())` alone doesn't catch this, since year 2
// AD is a legal JS Date. toUtcStartOfDay/toUtcEndOfDay zero-pad month/day but
// not the year, so a short year used to serialize straight into a malformed
// filter value (e.g. "2-01-10T00:00:00Z") and reach entity-service as a 400.
// Treating an incomplete year the same as an invalid one -- waiting for the
// rest of the digits rather than forwarding a technically-parseable but
// nonsensical date -- is the correct fix; padding it to "0002-01-10" would
// only make the malformed value syntactically valid, not correct.
//
// Every real caller of this component filters on "Created Date" or "Updated
// Date" (a support case/engagement can't have either in the future), so a
// future date is rejected here too, the same way `TimeCardsDateFilter`/
// `UsageMetricsTimeRangeSelector` already reject one via `isPastOrPresentDate`
// -- this component had no such guard at all before, on either the "From" or
// "To" side, found live from a real screenshot of the calendar popup
// happily offering every future day as clickable on the "From" field.
// Composed with the short-year check above into one `isComplete` override
// (below) passed to `useControlledDatePickerValue` for both fields -- see
// that hook's own doc comment for why `isComplete` is the right layer for a
// hand-typed date, and `disableFuture` on the `DatePicker` itself (below) is
// the matching visual layer so the calendar popup actually greys out and
// disables those days instead of silently swallowing a click on one.
function isCompleteCalendarDate(date: unknown): date is Date {
  return (
    date instanceof Date &&
    !isNaN(date.getTime()) &&
    date.getFullYear() >= 1000 &&
    isPastOrPresentDate(date)
  );
}

export type DateRangeFilterProps = {
  label: string;
  startDate: string | undefined;
  endDate: string | undefined;
  onStartChange: (val: string | undefined) => void;
  onEndChange: (val: string | undefined) => void;
};

/**
 * A pair of DatePickers rendered as a "From / To" date range filter.
 * Start is constrained to ≤ endDate and end is constrained to ≥ startDate.
 * Calls onStartChange / onEndChange with a UTC ISO string (YYYY-MM-DDTHH:MM:SSZ)
 * or undefined when the picker is cleared.
 */
export default function DateRangeFilter({
  label,
  startDate,
  endDate,
  onStartChange,
  onEndChange,
}: DateRangeFilterProps): JSX.Element {
  const start = useControlledDatePickerValue({
    value: startDate ?? "",
    onChange: (next) => onStartChange(next || undefined),
    parse: parseUtcIso,
    format: toUtcStartOfDay,
    isComplete: isCompleteCalendarDate,
  });
  const end = useControlledDatePickerValue({
    value: endDate ?? "",
    onChange: (next) => onEndChange(next || undefined),
    parse: parseUtcIsoEndDate,
    format: toUtcEndOfDay,
    isComplete: isCompleteCalendarDate,
  });

  return (
    <LocalizationProvider dateAdapter={AdapterDateFns}>
      <Box>
        <Typography
          variant="caption"
          color="text.secondary"
          sx={{ mb: 1.5, display: "block" }}
        >
          {label}
        </Typography>
        <Box
          sx={{
            display: "flex",
            gap: 1,
            flexDirection: { xs: "column", sm: "row" },
          }}
        >
          <DatePicker
            label="From"
            value={start.localDate}
            disableFuture
            maxDate={end.localDate ?? undefined}
            onChange={start.handleChange}
            slotProps={{
              textField: { size: "small", fullWidth: true },
              field: { clearable: true, onClear: start.handleClear },
            }}
          />
          <DatePicker
            label="To"
            value={end.localDate}
            disableFuture
            minDate={start.localDate ?? undefined}
            onChange={end.handleChange}
            slotProps={{
              textField: { size: "small", fullWidth: true },
              field: { clearable: true, onClear: end.handleClear },
            }}
          />
        </Box>
      </Box>
    </LocalizationProvider>
  );
}
