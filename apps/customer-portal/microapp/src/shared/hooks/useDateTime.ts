import { useCallback } from "react";

import dayjs from "dayjs";
import relativeTime from "dayjs/plugin/relativeTime";
import timezone from "dayjs/plugin/timezone";
import utc from "dayjs/plugin/utc";

import { useMe } from "@context/me";

dayjs.extend(utc);
dayjs.extend(timezone);
dayjs.extend(relativeTime);

export function useDateTime() {
  const { timezone } = useMe();
  const tz = timezone ?? Intl.DateTimeFormat().resolvedOptions().timeZone;

  const toUtc = useCallback((date: Date | string) => {
    if (date instanceof Date) {
      return dayjs.utc(dayjs(date).format("YYYY-MM-DDTHH:mm:ss"));
    }
    return dayjs.utc(date);
  }, []);

  const format = useCallback(
    (date: Date | string, fmt = "MMM D, YYYY h:mm A") => {
      return toUtc(date).tz(tz).format(fmt);
    },
    [toUtc, tz],
  );

  const fromNow = useCallback((date: Date | string) => {
    return dayjs.utc(date).fromNow();
  }, []);

  const toDate = useCallback(
    (date: Date | string) => {
      return toUtc(date).tz(tz).format("MMM D, YYYY");
    },
    [toUtc, tz],
  );

  const toTime = useCallback(
    (date: Date | string) => {
      return toUtc(date).tz(tz).format("h:mm A z");
    },
    [toUtc, tz],
  );

  return { format, fromNow, toDate, toTime };
}
