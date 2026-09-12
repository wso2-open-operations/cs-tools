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

import { AdapterDateFns, Box, DatePickers, Typography } from "@wso2/oxygen-ui";
import type { JSX } from "react";

const { DatePicker, LocalizationProvider } = DatePickers;

/** "YYYY-MM-DD" to a local-midnight Date (avoids the UTC-parse day-shift
 * `new Date(dateString)` can cause depending on the viewer's timezone) —
 * same helper `ChangeRequestsFilterBar` keeps locally for its own date-only
 * fields; duplicated here (rather than imported from that feature) since
 * this component lives in `csm-dashboard`, a lower-level feature than
 * `csm-operations` shouldn't depend on. */
function parseDateOnly(value: string | undefined): Date | null {
  if (!value) return null;
  const match = /^(\d{4})-(\d{2})-(\d{2})$/.exec(value);
  if (!match) return null;
  const date = new Date(Number(match[1]), Number(match[2]) - 1, Number(match[3]));
  return Number.isNaN(date.getTime()) ? null : date;
}

/** Local-midnight Date back to "YYYY-MM-DD". */
function formatDateOnly(date: Date): string {
  const y = date.getFullYear();
  const m = String(date.getMonth() + 1).padStart(2, "0");
  const d = String(date.getDate()).padStart(2, "0");
  return `${y}-${m}-${d}`;
}

export interface DateRangeFilterValue {
  /** "YYYY-MM-DD", inclusive. `undefined` means no lower bound. */
  from?: string;
  /** "YYYY-MM-DD", inclusive. `undefined` means no upper bound. */
  to?: string;
}

export interface DateRangeFilterProps {
  value: DateRangeFilterValue;
  onChange: (next: DateRangeFilterValue) => void;
  /** Shown above the two date fields. Defaults to a generic label — callers
   * with a more specific scope (e.g. "Feedback submitted") should override
   * it rather than leaving the generic one, which says nothing about WHAT
   * is being date-filtered. */
  label?: string;
}

/**
 * Reusable "from"/"to" date-range control, controlled by the caller (no
 * internal state) — the same pair-of-`DatePicker`s idiom every existing
 * filter bar in this app already hand-rolls per-feature (see
 * `ChangeRequestsFilterBar`'s own `closedStartDate`/`closedEndDate` fields),
 * extracted here as a real shared component because this is the first place
 * TWO different widget shapes (a list-shape grid and a bar-shape trend
 * chart — see `AgentsLandingPagePilot`) need the exact same date-range value
 * to drive two independent data fetches at once, rather than one filter bar
 * owning one table's own query.
 *
 * Each field clamps the OTHER field's own bound (`from`'s `maxDate` is
 * `to`, `to`'s `minDate` is `from`), same UX as every other date-range
 * picker pair in this app — this can't express an inverted range through
 * the UI at all, rather than accepting one and leaving a broader-than-
 * intended filter to be caught (or silently misread) downstream.
 */
export default function DateRangeFilter({
  value,
  onChange,
  label = "Date range",
}: DateRangeFilterProps): JSX.Element {
  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 1 }}>
      <Typography variant="caption" color="text.secondary">
        {label}
      </Typography>
      <Box sx={{ display: "flex", gap: 2, flexWrap: "wrap" }}>
        <LocalizationProvider dateAdapter={AdapterDateFns}>
          <DatePicker
            label="From"
            value={parseDateOnly(value.from)}
            maxDate={parseDateOnly(value.to) ?? undefined}
            onChange={(date) =>
              onChange({
                ...value,
                from:
                  date instanceof Date && !Number.isNaN(date.getTime())
                    ? formatDateOnly(date)
                    : undefined,
              })
            }
            slotProps={{
              textField: { size: "small" },
              field: { clearable: true },
            }}
          />
        </LocalizationProvider>
        <LocalizationProvider dateAdapter={AdapterDateFns}>
          <DatePicker
            label="To"
            value={parseDateOnly(value.to)}
            minDate={parseDateOnly(value.from) ?? undefined}
            onChange={(date) =>
              onChange({
                ...value,
                to:
                  date instanceof Date && !Number.isNaN(date.getTime())
                    ? formatDateOnly(date)
                    : undefined,
              })
            }
            slotProps={{
              textField: { size: "small" },
              field: { clearable: true },
            }}
          />
        </LocalizationProvider>
      </Box>
    </Box>
  );
}
