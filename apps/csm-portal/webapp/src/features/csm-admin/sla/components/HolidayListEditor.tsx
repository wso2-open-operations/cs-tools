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

import { Box, Button, IconButton, TextField, Typography } from "@wso2/oxygen-ui";
import { Plus, Trash2 } from "@wso2/oxygen-ui-icons-react";
import { useState, type JSX } from "react";
import type { SlaHoliday } from "../types";

interface Props {
  holidays: SlaHoliday[];
  onAdd: (input: { startDate: string; endDate: string; name: string }) => void;
  onDelete: (holidayId: string) => void;
  busy?: boolean;
}

/** A calendar's holiday list (R24): named date/range rows, add/remove. Not
 *  shared between calendars in v1 (open question 6). */
export default function HolidayListEditor({ holidays, onAdd, onDelete, busy }: Props): JSX.Element {
  const [startDate, setStartDate] = useState("");
  const [endDate, setEndDate] = useState("");
  const [name, setName] = useState("");

  const canAdd = Boolean(startDate && endDate && name.trim()) && !busy;

  const submit = (): void => {
    if (!canAdd) return;
    onAdd({ startDate, endDate, name: name.trim() });
    setStartDate("");
    setEndDate("");
    setName("");
  };

  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 1 }}>
      {holidays.length === 0 ? (
        <Typography variant="body2" color="text.secondary">
          No holidays set.
        </Typography>
      ) : (
        holidays
          .slice()
          .sort((a, b) => a.startDate.localeCompare(b.startDate))
          .map((h) => (
            <Box key={h.id} sx={{ display: "flex", alignItems: "center", gap: 2 }}>
              <Typography variant="body2" sx={{ width: 200 }}>
                {h.startDate === h.endDate ? h.startDate : `${h.startDate} – ${h.endDate}`}
              </Typography>
              <Typography variant="body2" sx={{ flex: 1 }}>
                {h.name}
              </Typography>
              <IconButton size="small" disabled={busy} onClick={() => onDelete(h.id)}>
                <Trash2 size={16} />
              </IconButton>
            </Box>
          ))
      )}
      <Box sx={{ display: "flex", alignItems: "center", gap: 2, mt: 1 }}>
        <TextField
          size="small"
          type="date"
          label="Start"
          value={startDate}
          onChange={(e) => setStartDate(e.target.value)}
        />
        <TextField size="small" type="date" label="End" value={endDate} onChange={(e) => setEndDate(e.target.value)} />
        <TextField
          size="small"
          label="Name"
          value={name}
          onChange={(e) => setName(e.target.value)}
          sx={{ flex: 1 }}
        />
        <Button size="small" startIcon={<Plus size={16} />} disabled={!canAdd} onClick={submit}>
          Add holiday
        </Button>
      </Box>
    </Box>
  );
}
