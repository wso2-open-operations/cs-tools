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

import { Box, Typography, Button, DatePickers, AdapterDateFns } from "@wso2/oxygen-ui";
import { Calendar, X } from "@wso2/oxygen-ui-icons-react";
import { format } from "date-fns";

const { LocalizationProvider, DatePicker } = DatePickers;
import type { JSX } from "react";
import type { TimeCardsDateFilterProps } from "@features/project-details/types/projectDetailsComponents";

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
 * TimeCardsDateFilter provides a compact time-range filter for time cards.
 *
 * @param {TimeCardsDateFilterProps} props - Date values and change handlers.
 * @returns {JSX.Element} The rendered filter row.
 */
export default function TimeCardsDateFilter({
  startDate,
  endDate,
  onStartDateChange,
  onEndDateChange,
  onClear,
}: TimeCardsDateFilterProps): JSX.Element {
  const hasFilters = Boolean(startDate || endDate);
  const parsedStart = parseDateOnly(startDate);
  const parsedEnd = parseDateOnly(endDate);

  return (
    <LocalizationProvider dateAdapter={AdapterDateFns}>
      <Box sx={{ display: "flex", alignItems: "center", gap: 1.5, flexWrap: "wrap" }}>
        <Calendar size={18} />
        <Typography variant="body2" sx={{ fontWeight: 600 }}>
          Case Created Date :
        </Typography>
        <Box sx={{ display: "flex", alignItems: "center", gap: 1, flexWrap: "wrap" }}>
          <DatePicker
            label="From"
            value={parsedStart}
            disableFuture
            maxDate={parsedEnd ?? undefined}
            onChange={(date) => {
              onStartDateChange(date instanceof Date && !isNaN(date.getTime()) ? formatDateOnly(date) : "");
            }}
            slotProps={{
              textField: {
                id: "time-cards-start-date",
                size: "small",
                sx: { minWidth: 160 },
                slotProps: { htmlInput: { "aria-label": "Start date" } },
              },
              field: { clearable: true },
            }}
          />
          <DatePicker
            label="To"
            value={parsedEnd}
            disableFuture
            minDate={parsedStart ?? undefined}
            onChange={(date) => {
              onEndDateChange(date instanceof Date && !isNaN(date.getTime()) ? formatDateOnly(date) : "");
            }}
            slotProps={{
              textField: {
                id: "time-cards-end-date",
                size: "small",
                sx: { minWidth: 160 },
                slotProps: { htmlInput: { "aria-label": "End date" } },
              },
              field: { clearable: true },
            }}
          />
          {hasFilters && onClear && (
            <Button
              variant="text"
              size="small"
              onClick={onClear}
              startIcon={<X size={16} />}
              sx={{ color: "text.secondary" }}
            >
              Clear
            </Button>
          )}
        </Box>
      </Box>
    </LocalizationProvider>
  );
}
