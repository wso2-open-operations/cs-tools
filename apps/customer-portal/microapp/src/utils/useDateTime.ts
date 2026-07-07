import { useCallback } from "react";
import dayjs from "dayjs";
import utc from "dayjs/plugin/utc";
import timezone from "dayjs/plugin/timezone";
import relativeTime from "dayjs/plugin/relativeTime";
import { useMe } from "../context/me";

dayjs.extend(utc);
dayjs.extend(timezone);
dayjs.extend(relativeTime);

/**
 * Returns the given timezone if it is a valid IANA identifier, otherwise
 * falls back to the device timezone. Guards against placeholder values
 * (e.g. ServiceNow's "--None--") that would make Intl/dayjs throw.
 */
const resolveTimezone = (tz?: string): string => {
  const fallback = Intl.DateTimeFormat().resolvedOptions().timeZone;
  if (!tz) return fallback;
  try {
    Intl.DateTimeFormat(undefined, { timeZone: tz });
    return tz;
  } catch {
    return fallback;
  }
};

export function useDateTime() {
  const { timezone } = useMe();
  const tz = resolveTimezone(timezone);

  const toUtc = useCallback((date: Date | string) => {
    if (date instanceof Date) {
      return dayjs.utc(dayjs(date).format("YYYY-MM-DDTHH:mm:ss"));
    }
    return dayjs.utc(date);
  }, []);

  const format = useCallback(
    (date: Date | string, fmt = "MMM D, YYYY h:mm A") => toUtc(date).tz(tz).format(fmt),
    [toUtc, tz],
  );

  const fromNow = useCallback((date: Date | string) => toUtc(date).tz(tz).fromNow(), [toUtc, tz]);

  const toDate = useCallback((date: Date | string) => toUtc(date).tz(tz).format("MMM D, YYYY"), [toUtc, tz]);

  const toTime = useCallback((date: Date | string) => toUtc(date).tz(tz).format("h:mm A z"), [toUtc, tz]);

  return { format, fromNow, toDate, toTime };
}
