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

// Ported from apps/support-portal-lite/webapp's TeamSchedulePage (itself
// via one-wso2, from SupportPortalLite's ScheduleTable.tsx). UI/JSX
// unchanged; data layer rewritten onto useBackendApi()/React Query — see
// useGetTeamSchedule.ts. No SplShell/PageHeader wrapper — this app's own
// AppLayout/RouteGuard already provide the page chrome. Routed at both
// spl/team-schedule and spl/team-schedule/:sysId (see App.tsx).
import React, { useState, type JSX } from "react";
import {
  AdapterDateFns,
  Box,
  Button,
  DatePickers,
  FormControl,
  InputLabel,
  MenuItem,
  Paper,
  Select,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TableRow,
  Tooltip,
  tooltipClasses,
  Typography,
  type TooltipProps,
} from "@wso2/oxygen-ui";
import { styled } from "@mui/material/styles";
import { useParams } from "react-router";

// oxygen-ui's own bundled date-fns adapter/pickers (see CsmTimeCardsPage.tsx
// for the precedent) — avoids adding dayjs as a new dependency just for this
// one ported page, unlike the source app which used @mui/x-date-pickers'
// AdapterDayjs directly.
const { DesktopDatePicker: DatePicker, LocalizationProvider } = DatePickers;

/** "YYYY-MM-DD" to a local-midnight Date (avoids the UTC-parse day-shift
 * `new Date(dateString)` can cause depending on the viewer's timezone). */
function parseDateOnly(value: string): Date | null {
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
import { safeRouteId } from "@features/spl/utils/routeId";
import { BackendApiError } from "@api/backend/client";
import { useGetTeamSchedule } from "@features/spl/schedule/api/useGetTeamSchedule";
import type { ABTTeamScheduleList } from "@features/spl/schedule/scheduleTypes";
import { openExternalUrl } from "@utils/openExternalUrl";
import "@features/spl/schedule/ScheduleTable.css";

enum EventType {
  Default = "ops_default",
  Engagement = "engagement",
  Evening = "ops_evening",
  Exclude = "exclude",
  TimeOff = "time_off",
  TimeOffEvening = "time_off_evening",
  TimeOffMorning = "time_off_morning",
  Morning = "ops_morning",
  Night = "ops_night",
  WeekendNight = "ops_weekend_night",
  Weekend = "ops_weekend",
}

function getBackgroundColor(eventType: string): string {
  switch (eventType) {
    case EventType.Default:
      return "#F0EAE6";
    case EventType.Engagement:
      return "#E6F0EA";
    case EventType.Evening:
      return "#E1F8DC";
    case EventType.Exclude:
    case EventType.TimeOff:
    case EventType.TimeOffEvening:
    case EventType.TimeOffMorning:
      return "#F7D8BA";
    case EventType.Morning:
      return "#ACDDDE";
    case EventType.Night:
    case EventType.WeekendNight:
      return "#CAF1DE";
    case EventType.Weekend:
      return "#FFE7C7";
    default:
      return "#F0EAE6";
  }
}

const WEEKDAY_LABELS = ["Sun", "Mon", "Tue", "Wed", "Thu", "Fri", "Sat"];
const MONTH_LABELS = [
  "Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep", "Oct", "Nov", "Dec",
];

/** "YYYY-MM-DD" to "ddd, D MMM" (e.g. "Mon, 1 Jan") — matches the source
 * app's dayjs format exactly, without adding dayjs as a dependency. */
function formatDates(dates: string[]): string[] {
  return dates.map((dateStr) => {
    const parsed = parseDateOnly(dateStr);
    if (!parsed) return dateStr;
    return `${WEEKDAY_LABELS[parsed.getDay()]}, ${parsed.getDate()} ${MONTH_LABELS[parsed.getMonth()]}`;
  });
}

function getUniqueDates(list: ABTTeamScheduleList[]): string[] {
  const datesSet = new Set<string>();
  list.forEach((listItem) => {
    listItem.members.forEach((member) => {
      Object.keys(member.schedule).forEach((date) => datesSet.add(date));
    });
  });
  return Array.from(datesSet);
}

const BlackTooltip = styled(({ className, ...props }: TooltipProps) => (
  <Tooltip {...props} arrow classes={{ popper: className }} />
))(() => ({
  [`& .${tooltipClasses.arrow}`]: {
    color: "#212A30",
  },
  [`& .${tooltipClasses.tooltip}`]: {
    backgroundColor: "#212A30",
    fontSize: "12px",
  },
}));

export default function TeamSchedulePage(): JSX.Element {
  const { sysId } = useParams<{ sysId?: string }>();
  const [teamId, setTeamId] = useState(safeRouteId(sysId));
  const [duration, setDuration] = useState("");
  const [from, setFrom] = useState<string>(formatDateOnly(new Date()));
  const [eventType, setEventType] = useState("");

  // Keep teamId synced with the routed :sysId — without this, navigating
  // between /spl/team-schedule/:sysId links while this page stays mounted
  // (same component, React preserves its state) would leave the selector,
  // request, and ServiceNow link pointing at the previous team. Adjusting
  // state during render (React's documented pattern for this, see
  // https://react.dev/learn/you-might-not-need-an-effect) instead of an
  // effect, so it resolves in the same commit rather than triggering an
  // extra render.
  const [prevSysId, setPrevSysId] = useState(sysId);
  if (sysId !== prevSysId) {
    setPrevSysId(sysId);
    setTeamId(safeRouteId(sysId));
  }

  const { data, isLoading, error } = useGetTeamSchedule({ teamId, duration, from, eventType });

  const serviceNowUrl = (data?.snURL ?? "") + teamId;

  const uniqueDates = getUniqueDates(data?.list ?? []);
  const headerDates = formatDates(uniqueDates);

  const handleFromChange = (newValue: Date | null) => {
    if (newValue) setFrom(formatDateOnly(newValue));
  };

  const handleSNUrlClick = () => {
    openExternalUrl(serviceNowUrl);
  };

  return (
    <Box className="schedule-app-container">
      <Typography variant="h5" sx={{ mb: 2 }}>
        Team schedule
      </Typography>
      <Box sx={{ display: "flex", gap: 4, alignItems: "center", margin: "15px" }}>
        <Box sx={{ display: "flex", gap: 4, alignItems: "center", margin: "15px", width: "70%" }}>
          <FormControl sx={{ minWidth: 120 }}>
            <InputLabel id="schedule-team-label" sx={{ fontWeight: "bold" }}>
              Team
            </InputLabel>
            <Select
              labelId="schedule-team-label"
              value={teamId}
              label="Team"
              onChange={(e) => setTeamId(e.target.value)}
              autoWidth
            >
              <MenuItem value="">
                <em>All</em>
              </MenuItem>
              {data?.metadata[0]?.teams.map((team) => (
                <MenuItem value={team.id} key={team.id}>
                  {team.label}
                </MenuItem>
              ))}
            </Select>
          </FormControl>

          <FormControl sx={{ minWidth: 180 }}>
            <InputLabel id="schedule-allocation-label" sx={{ fontWeight: "bold" }}>
              Allocation Type
            </InputLabel>
            <Select
              labelId="schedule-allocation-label"
              value={eventType}
              label="Allocation Type"
              onChange={(e) => setEventType(e.target.value)}
              autoWidth
            >
              <MenuItem value="">
                <em>None</em>
              </MenuItem>
              {data?.metadata[0]?.eventTypes.map((event) => (
                <MenuItem value={event.name} key={event.name}>
                  {event.label}
                </MenuItem>
              ))}
            </Select>
          </FormControl>

          <FormControl sx={{ minWidth: 120 }}>
            <InputLabel id="schedule-duration-label" sx={{ fontWeight: "bold" }}>
              Duration
            </InputLabel>
            <Select
              labelId="schedule-duration-label"
              value={duration}
              label="Duration"
              onChange={(e) => setDuration(e.target.value)}
              autoWidth
            >
              <MenuItem value="1">1 Week</MenuItem>
              <MenuItem value="2">2 Weeks</MenuItem>
              <MenuItem value="3">3 Weeks</MenuItem>
              <MenuItem value="4">4 Weeks</MenuItem>
            </Select>
          </FormControl>

          <LocalizationProvider dateAdapter={AdapterDateFns}>
            <DatePicker label="From" value={parseDateOnly(from)} onChange={handleFromChange} />
          </LocalizationProvider>
        </Box>
        <Box sx={{ display: "flex", justifyContent: "flex-end", marginLeft: "35px" }} width="20%">
          <BlackTooltip title="ServiceNow access is required" placement="top" arrow>
            <Button
              size="small"
              onClick={handleSNUrlClick}
              sx={{
                color: "#e96900",
                fontSize: "15px",
                marginLeft: "-5px",
                fontWeight: "bold",
                ":hover": { bgcolor: "#e96900", borderColor: "primary.main", color: "white" },
              }}
            >
              Open in ServiceNow
            </Button>
          </BlackTooltip>
        </Box>
      </Box>

      {isLoading ? (
        <Box sx={{ display: "flex", justifyContent: "center", mt: 4 }}>
          <Typography variant="body1" color="text.secondary">
            Loading…
          </Typography>
        </Box>
      ) : error ? (
        <Typography sx={{ mt: 4, textAlign: "center" }} color="error">
          {error instanceof BackendApiError && error.status === 404
            ? "Item not found."
            : "Something went wrong."}
        </Typography>
      ) : (
        data && (
          <TableContainer component={Paper} className="schedule-table-container">
            <Table stickyHeader>
              <TableHead className="schedule-table-header">
                <TableRow>
                  <TableCell className="schedule-header-cell">Team</TableCell>
                  <TableCell className="schedule-header-cell">Member</TableCell>
                  {headerDates.map((day) => (
                    <TableCell key={day} className="schedule-header-cell">
                      {day}
                    </TableCell>
                  ))}
                </TableRow>
              </TableHead>
              <TableBody>
                {data.list.map((team, teamIndex) => (
                  <React.Fragment key={teamIndex}>
                    {team.members.map((member, memberIndex) => (
                      <TableRow key={memberIndex}>
                        {memberIndex === 0 && (
                          <TableCell
                            rowSpan={team.members.length}
                            className="schedule-team-cell schedule-sticky-column"
                          >
                            {team.label}
                          </TableCell>
                        )}
                        <TableCell className="schedule-team-cell schedule-sticky-column">
                          {member.name}
                          {member.roles[0] && (
                            <span
                              style={{
                                marginLeft: "8px",
                                padding: "3px 8px",
                                backgroundColor: "#e0f7fa",
                                borderRadius: "10px",
                                fontSize: "0.8rem",
                                fontWeight: "bold",
                                color: "#036300",
                              }}
                            >
                              {member.roles[0].label}
                            </span>
                          )}
                        </TableCell>
                        {uniqueDates.map((day) => (
                          <TableCell key={day} className="schedule-table-cell">
                            {member.schedule[day] ? (
                              member.schedule[day].map((schedule, scheduleIndex) => (
                                <span
                                  key={scheduleIndex}
                                  style={{
                                    marginLeft: "8px",
                                    padding: "4px 40px",
                                    backgroundColor: getBackgroundColor(schedule.name),
                                    borderRadius: "10px",
                                    fontSize: "0.85rem",
                                    color: "black",
                                    display: "inline-block",
                                    marginBottom: "4px",
                                  }}
                                >
                                  {schedule.label}
                                </span>
                              ))
                            ) : (
                              <span
                                style={{
                                  marginLeft: "8px",
                                  padding: "4px 40px",
                                  backgroundColor: "#F0EAE6",
                                  borderRadius: "10px",
                                  fontSize: "0.85rem",
                                  color: "black",
                                }}
                              >
                                Default
                              </span>
                            )}
                          </TableCell>
                        ))}
                      </TableRow>
                    ))}
                  </React.Fragment>
                ))}
              </TableBody>
            </Table>
          </TableContainer>
        )
      )}
    </Box>
  );
}
