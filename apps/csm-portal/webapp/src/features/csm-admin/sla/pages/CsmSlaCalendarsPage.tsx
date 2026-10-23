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

import { Alert, Box, Button, Divider, Skeleton, Typography } from "@wso2/oxygen-ui";
import { useState, type JSX } from "react";
import {
  useAddSlaHoliday,
  useDeleteSlaHoliday,
  useSlaCalendars,
  useUpdateSlaCalendarWorkingWeek,
} from "../api/useSlaCalendars";
import type { SlaCalendar, SlaWorkingDay } from "../types";
import HolidayListEditor from "../components/HolidayListEditor";
import WorkingWeekEditor from "../components/WorkingWeekEditor";

function CalendarEditor({ calendar }: { calendar: SlaCalendar }): JSX.Element {
  const updateWeek = useUpdateSlaCalendarWorkingWeek();
  const addHoliday = useAddSlaHoliday();
  const deleteHoliday = useDeleteSlaHoliday();
  const [week, setWeek] = useState<SlaWorkingDay[]>(calendar.workingWeek);
  // Tracks which server copy `week` was last seeded from, so a save (which
  // refetches the calendar) re-seeds the local draft — same "server is the
  // source of truth, local state is just the in-progress edit" posture as
  // every other admin editor in this app. Adjusting state during render
  // (not in an effect) per React's own guidance for "reset state when a
  // prop changes" — see react-hooks/set-state-in-effect.
  const [syncedFrom, setSyncedFrom] = useState(calendar.workingWeek);
  if (calendar.workingWeek !== syncedFrom) {
    setSyncedFrom(calendar.workingWeek);
    setWeek(calendar.workingWeek);
  }

  const dirty = JSON.stringify(week) !== JSON.stringify(calendar.workingWeek);

  return (
    <Box sx={{ mb: 4 }}>
      <Typography variant="h6">{calendar.name}</Typography>
      <Typography variant="body2" color="text.secondary" sx={{ mb: 2 }}>
        Hours are local wall-clock time — the SLA definition that uses this calendar decides which time zone they
        are read in.
      </Typography>
      <WorkingWeekEditor value={week} onChange={setWeek} disabled={updateWeek.isPending} />
      <Box sx={{ mt: 1 }}>
        {dirty ? (
          <Button
            variant="contained"
            size="small"
            disabled={updateWeek.isPending}
            onClick={() => updateWeek.mutate({ id: calendar.id, workingWeek: week })}
          >
            Save working week
          </Button>
        ) : null}
      </Box>
      {updateWeek.isError ? <Alert severity="error">Failed to save the working week.</Alert> : null}

      <Typography variant="subtitle1" sx={{ mt: 3, mb: 1 }}>
        Holidays
      </Typography>
      <HolidayListEditor
        holidays={calendar.holidays}
        busy={addHoliday.isPending || deleteHoliday.isPending}
        onAdd={(input) => addHoliday.mutate({ calendarId: calendar.id, ...input })}
        onDelete={(holidayId) => deleteHoliday.mutate({ calendarId: calendar.id, holidayId })}
      />
      {addHoliday.isError ? <Alert severity="error">Failed to add the holiday.</Alert> : null}
    </Box>
  );
}

/**
 * Calendars tab (R22-R27): the two seeded calendars (24x7, WSO2 Working
 * Days) only — no create/retire flow, see this feature's own plan file,
 * "Calendars are two seeded rows."
 */
export default function CsmSlaCalendarsPage(): JSX.Element {
  const calendars = useSlaCalendars();

  if (calendars.isLoading) return <Skeleton variant="rectangular" height={400} />;
  if (calendars.isError || !calendars.data) return <Alert severity="error">Failed to load SLA calendars.</Alert>;

  return (
    <Box>
      {calendars.data.map((c, idx) => (
        <Box key={c.id}>
          <CalendarEditor calendar={c} />
          {idx < calendars.data.length - 1 ? <Divider sx={{ mb: 3 }} /> : null}
        </Box>
      ))}
    </Box>
  );
}
