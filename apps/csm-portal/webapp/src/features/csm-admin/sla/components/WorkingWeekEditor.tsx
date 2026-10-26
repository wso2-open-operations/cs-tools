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

import { Box, Switch, TextField, Typography } from "@wso2/oxygen-ui";
import type { JSX } from "react";
import type { SlaWorkingDay } from "../types";

/** 0 = Sunday .. 6 = Saturday, matching SlaWorkingDay.weekday (R23). */
const WEEKDAY_LABELS = ["Sunday", "Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday"];

interface Props {
  value: SlaWorkingDay[];
  onChange: (next: SlaWorkingDay[]) => void;
  disabled?: boolean;
}

/**
 * The weekly working-hours grid (R23): 7 rows, a working toggle plus
 * start/end time per day. No weekend is assumed — every day starts
 * editable, since a calendar may work e.g. Sunday-Thursday.
 */
export default function WorkingWeekEditor({ value, onChange, disabled }: Props): JSX.Element {
  const dayOf = (weekday: number): SlaWorkingDay =>
    value.find((d) => d.weekday === weekday) ?? { weekday, isWorking: false, startTime: null, endTime: null };

  const update = (weekday: number, patch: Partial<SlaWorkingDay>): void => {
    const next = WEEKDAY_LABELS.map((_, w) => {
      const day = dayOf(w);
      return w === weekday ? { ...day, ...patch } : day;
    });
    onChange(next);
  };

  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 1 }}>
      {WEEKDAY_LABELS.map((label, weekday) => {
        const day = dayOf(weekday);
        return (
          <Box key={weekday} sx={{ display: "flex", alignItems: "center", gap: 2 }}>
            <Typography variant="body2" sx={{ width: 110 }}>
              {label}
            </Typography>
            <Switch
              checked={day.isWorking}
              disabled={disabled}
              onChange={(e) =>
                update(weekday, {
                  isWorking: e.target.checked,
                  startTime: e.target.checked ? (day.startTime ?? "09:00") : null,
                  endTime: e.target.checked ? (day.endTime ?? "17:00") : null,
                })
              }
            />
            {day.isWorking ? (
              <>
                <TextField
                  size="small"
                  type="time"
                  value={day.startTime ?? "09:00"}
                  disabled={disabled}
                  onChange={(e) => update(weekday, { startTime: e.target.value })}
                />
                <Typography variant="body2">to</Typography>
                <TextField
                  size="small"
                  type="time"
                  value={day.endTime ?? "17:00"}
                  disabled={disabled}
                  onChange={(e) => update(weekday, { endTime: e.target.value })}
                />
              </>
            ) : (
              <Typography variant="body2" color="text.secondary">
                Not a working day
              </Typography>
            )}
          </Box>
        );
      })}
    </Box>
  );
}
