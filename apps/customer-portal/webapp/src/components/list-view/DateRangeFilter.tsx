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
  const parsedStart = parseUtcIso(startDate);
  const parsedEnd = parseUtcIsoEndDate(endDate);

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
            value={parsedStart}
            maxDate={parsedEnd ?? undefined}
            onChange={(date) => {
              onStartChange(date instanceof Date && !isNaN(date.getTime()) ? toUtcStartOfDay(date) : undefined);
            }}
            slotProps={{
              textField: { size: "small", fullWidth: true },
              field: { clearable: true },
            }}
          />
          <DatePicker
            label="To"
            value={parsedEnd}
            minDate={parsedStart ?? undefined}
            onChange={(date) => {
              onEndChange(date instanceof Date && !isNaN(date.getTime()) ? toUtcEndOfDay(date) : undefined);
            }}
            slotProps={{
              textField: { size: "small", fullWidth: true },
              field: { clearable: true },
            }}
          />
        </Box>
      </Box>
    </LocalizationProvider>
  );
}
