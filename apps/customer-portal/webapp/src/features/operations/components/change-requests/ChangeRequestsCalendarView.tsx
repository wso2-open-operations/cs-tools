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
  Typography,
  Paper,
  colors,
  alpha,
  IconButton,
} from "@wso2/oxygen-ui";
import {
  Calendar,
  ChevronLeft,
  ChevronRight,
} from "@wso2/oxygen-ui-icons-react";
import { useState } from "react";
import type { JSX } from "react";
import type { ChangeRequestItem } from "@features/operations/types/changeRequests";
import ChangeRequestsCalendarSkeleton from "@features/operations/components/change-requests/ChangeRequestsCalendarSkeleton";
import error500Svg from "@assets/error/error-500.svg";
import { getChangeRequestStateColor } from "@features/operations/utils/changeRequestUi";
import type { ChangeRequestsCalendarViewProps } from "@features/operations/types/changeRequests";
import {
  CHANGE_REQUEST_CALENDAR_LEGEND_STATES,
  CHANGE_REQUEST_CALENDAR_WEEKDAY_LABELS,
} from "@features/operations/constants/operationsConstants";
import { parseBackendTimestamp, resolveDisplayTimeZone } from "@utils/dateTime";

/**
 * Calendar view for change requests.
 *
 * @param {ChangeRequestsCalendarViewProps} props - Change requests and handlers.
 * @returns {JSX.Element} The rendered calendar view.
 */
export default function ChangeRequestsCalendarView({
  changeRequests,
  isLoading,
  isError = false,
  onChangeRequestClick,
}: ChangeRequestsCalendarViewProps): JSX.Element {
  const displayTimeZone = resolveDisplayTimeZone();
  const getZonedDateParts = (date: Date) => {
    const parts = new Intl.DateTimeFormat("en-US", {
      timeZone: displayTimeZone,
      year: "numeric",
      month: "numeric",
      day: "numeric",
      hour: "numeric",
      minute: "2-digit",
      hour12: true,
    }).formatToParts(date);
    const getPart = (type: Intl.DateTimeFormatPartTypes) =>
      parts.find((part) => part.type === type)?.value ?? "";
    const year = Number(getPart("year"));
    const month = Number(getPart("month"));
    const day = Number(getPart("day"));
    const hour = getPart("hour");
    const minute = getPart("minute");
    const dayPeriod = getPart("dayPeriod");
    return {
      year,
      monthIndex: month - 1,
      day,
      timeLabel:
        hour && minute && dayPeriod
          ? `${hour}:${minute} ${dayPeriod.toUpperCase()}`
          : "--",
    };
  };

  // Compute initial month/year from first valid request or use current date
  const getInitialMonthYear = () => {
    const zonedNow = getZonedDateParts(new Date());
    if (changeRequests.length > 0) {
      const firstValidRequest = changeRequests.find((req) => {
        if (!req.startDate) return false;
        return parseBackendTimestamp(req.startDate) !== null;
      });

      if (firstValidRequest) {
        const date = parseBackendTimestamp(firstValidRequest.startDate);
        if (!date) return { month: zonedNow.monthIndex, year: zonedNow.year };
        const zoned = getZonedDateParts(date);
        return { month: zoned.monthIndex, year: zoned.year };
      }
    }
    return { month: zonedNow.monthIndex, year: zonedNow.year };
  };

  const [currentMonth, setCurrentMonth] = useState(getInitialMonthYear().month);
  const [currentYear, setCurrentYear] = useState(getInitialMonthYear().year);

  const handlePreviousMonth = () => {
    if (currentMonth === 0) {
      setCurrentMonth(11);
      setCurrentYear(currentYear - 1);
    } else {
      setCurrentMonth(currentMonth - 1);
    }
  };

  const handleNextMonth = () => {
    if (currentMonth === 11) {
      setCurrentMonth(0);
      setCurrentYear(currentYear + 1);
    } else {
      setCurrentMonth(currentMonth + 1);
    }
  };

  if (isLoading) {
    return <ChangeRequestsCalendarSkeleton />;
  }

  if (isError) {
    return (
      <Box sx={{ textAlign: "center", py: 6 }}>
        <img src={error500Svg} alt="" aria-hidden="true" style={{ width: 200, height: "auto" }} />
        <Typography variant="body2" color="text.secondary" sx={{ mt: 3 }}>
          Failed to load calendar. Please try again.
        </Typography>
      </Box>
    );
  }

  // Use state-managed month and year
  const today = getZonedDateParts(new Date());
  const todayDate = today.day;
  const todayMonth = today.monthIndex;
  const todayYear = today.year;

  // Get first day of month and number of days
  const firstDayOfMonth = new Date(currentYear, currentMonth, 1);
  const lastDayOfMonth = new Date(currentYear, currentMonth + 1, 0);
  const daysInMonth = lastDayOfMonth.getDate();
  const startDayOfWeek = firstDayOfMonth.getDay(); // 0 = Sunday

  // Group change requests by day of month
  const requestsByDay: Record<number, ChangeRequestItem[]> = {};
  changeRequests.forEach((item) => {
    // Skip items without valid startDate
    if (!item.startDate) return;

    // Parse date - handle "YYYY-MM-DD HH:mm:ss" format from API
    const date = parseBackendTimestamp(item.startDate);

    // Skip invalid dates
    if (!date) return;

    const zoned = getZonedDateParts(date);
    if (zoned.monthIndex === currentMonth && zoned.year === currentYear) {
      const day = zoned.day;
      if (!requestsByDay[day]) {
        requestsByDay[day] = [];
      }
      requestsByDay[day].push(item);
    }
  });

  // Create calendar cells
  const calendarCells: JSX.Element[] = [];
  const totalCells = Math.ceil((startDayOfWeek + daysInMonth) / 7) * 7;

  for (let i = 0; i < totalCells; i++) {
    const dayNumber = i - startDayOfWeek + 1;
    const isValidDay = dayNumber > 0 && dayNumber <= daysInMonth;
    const isToday =
      isValidDay &&
      dayNumber === todayDate &&
      currentMonth === todayMonth &&
      currentYear === todayYear;
    const dayRequests = isValidDay ? requestsByDay[dayNumber] || [] : [];

    calendarCells.push(
      <Box
        key={i}
        sx={{
          minHeight: 128,
          border: isValidDay ? "1px solid" : "none",
          borderColor: isToday ? colors.blue[300] : "divider",
          bgcolor: isToday
            ? alpha(colors.blue[500], 0.05)
            : isValidDay
              ? "background.paper"
              : "action.hover",
          p: isValidDay ? 1 : 0,
          overflow: "auto",
          "&::-webkit-scrollbar": {
            width: "4px",
          },
          "&::-webkit-scrollbar-thumb": {
            bgcolor: "divider",
            borderRadius: "4px",
          },
        }}
      >
        {isValidDay && (
          <>
            <Typography
              variant="body2"
              sx={{
                mb: 1,
                color: isToday ? colors.blue[600] : "text.primary",
                fontWeight: isToday ? 600 : 400,
              }}
            >
              {dayNumber}
            </Typography>
            <Box sx={{ display: "flex", flexDirection: "column", gap: 0.5 }}>
              {dayRequests.map((item) => {
                const scheduledTime = parseBackendTimestamp(item.startDate);
                if (!scheduledTime) return null;
                const timeStr = getZonedDateParts(scheduledTime).timeLabel;
                const stateColor = getChangeRequestStateColor(item.state);

                return (
                  <Box
                    key={item.id}
                    component="button"
                    onClick={(e) => {
                      e.stopPropagation();
                      onChangeRequestClick?.(item);
                    }}
                    sx={{
                      fontSize: "0.75rem",
                      p: 0.75,
                      borderRadius: 1,
                      cursor: "pointer",
                      border: "1px solid",
                      bgcolor: alpha(stateColor, 0.1),
                      color: stateColor,
                      borderColor: alpha(stateColor, 0.3),
                      transition: "opacity 0.2s",
                      "&:hover": {
                        opacity: 0.8,
                      },
                      "&:focus": {
                        outline: "2px solid",
                        outlineColor: stateColor,
                        outlineOffset: "2px",
                      },
                      textAlign: "left",
                      width: "100%",
                      display: "block",
                    }}
                    title={`${item.title} - ${item.state?.label || "No Status"}`}
                    aria-label={`${item.number}: ${item.title} - ${item.state?.label || "No Status"}`}
                  >
                    <Box
                      sx={{
                        overflow: "hidden",
                        textOverflow: "ellipsis",
                        whiteSpace: "nowrap",
                        fontWeight: 500,
                      }}
                    >
                      {item.number}
                    </Box>
                    <Box
                      sx={{
                        overflow: "hidden",
                        textOverflow: "ellipsis",
                        whiteSpace: "nowrap",
                        fontSize: "0.7rem",
                        mt: 0.25,
                      }}
                      title={item.title}
                    >
                      {item.title}
                    </Box>
                    <Box sx={{ fontSize: "0.7rem", opacity: 0.75, mt: 0.25 }}>
                      {timeStr}
                    </Box>
                  </Box>
                );
              })}
            </Box>
          </>
        )}
      </Box>,
    );
  }

  // Use a UTC anchor date to avoid browser-local month drift when formatting in another timezone.
  const monthHeaderAnchor = new Date(Date.UTC(currentYear, currentMonth, 15, 12, 0, 0));
  const monthName = monthHeaderAnchor.toLocaleDateString("en-US", {
    month: "long",
    year: "numeric",
    timeZone: displayTimeZone,
  });

  if (changeRequests.length === 0) {
    return (
      <Box sx={{ textAlign: "center", py: 6 }}>
        <Calendar size={48} style={{ opacity: 0.3, marginBottom: 16 }} />
        <Typography variant="body1" color="text.secondary">
          No change requests scheduled.
        </Typography>
      </Box>
    );
  }

  return (
    <Paper
      sx={{
        p: 0,
        overflow: "hidden",
      }}
    >
      {/* Header */}
      <Box sx={{ px: 3, pt: 3 }}>
        <Typography variant="h6" color="text.primary" sx={{ mb: 0.5 }}>
          {monthName}
        </Typography>
        <Typography variant="body2" color="text.secondary">
          Click on a change request to view details
        </Typography>
      </Box>

      {/* Calendar Grid */}
      <Box sx={{ px: 3, pb: 3, pt: 2 }}>
        <Box
          sx={{
            display: "grid",
            gridTemplateColumns: "repeat(7, 1fr)",
            gap: 0,
            border: "1px solid",
            borderColor: "divider",
            borderRadius: 1,
            overflow: "hidden",
          }}
        >
          {/* Day headers */}
          {CHANGE_REQUEST_CALENDAR_WEEKDAY_LABELS.map((day) => (
            <Box
              key={day}
              sx={{
                p: 1,
                textAlign: "center",
                fontSize: "0.875rem",
                bgcolor: "action.hover",
                borderRight: "1px solid",
                borderBottom: "1px solid",
                borderColor: "divider",
                "&:last-child": {
                  borderRight: "none",
                },
              }}
            >
              {day}
            </Box>
          ))}

          {/* Calendar cells */}
          {calendarCells}
        </Box>

        {/* Legend */}
        <Box
          sx={{
            display: "flex",
            alignItems: "center",
            justifyContent: "space-between",
            gap: 2,
            mt: 2,
            pt: 2,
            borderTop: "1px solid",
            borderColor: "divider",
          }}
        >
          <Box
            sx={{
              display: "flex",
              alignItems: "center",
              gap: 1.5,
              flexWrap: "wrap",
            }}
          >
            <Typography variant="body2" color="text.secondary">
              States:
            </Typography>
            {CHANGE_REQUEST_CALENDAR_LEGEND_STATES.map((state) => {
              const c = getChangeRequestStateColor(state);
              return (
                <Box
                  key={state}
                  sx={{ display: "flex", alignItems: "center", gap: 0.5 }}
                >
                  <Box
                    sx={{
                      width: 12,
                      height: 12,
                      borderRadius: 0.5,
                      bgcolor: alpha(c, 3),
                      border: "1px solid",
                      borderColor: alpha(c, 0.3),
                    }}
                  />
                  <Typography variant="caption" color="text.secondary">
                    {state}
                  </Typography>
                </Box>
              );
            })}
          </Box>

          {/* Month Navigation */}
          <Box sx={{ display: "flex", alignItems: "center", gap: 0.5 }}>
            <IconButton
              size="small"
              onClick={handlePreviousMonth}
              sx={{ color: colors.grey[600] }}
              aria-label="Previous month"
            >
              <ChevronLeft />
            </IconButton>
            <IconButton
              size="small"
              onClick={handleNextMonth}
              sx={{ color: colors.grey[600] }}
              aria-label="Next month"
            >
              <ChevronRight />
            </IconButton>
          </Box>
        </Box>
      </Box>
    </Paper>
  );
}
