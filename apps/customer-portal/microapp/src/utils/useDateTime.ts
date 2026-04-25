import dayjs from "dayjs";
import utc from "dayjs/plugin/utc";
import timezone from "dayjs/plugin/timezone";
import relativeTime from "dayjs/plugin/relativeTime";
import { useMe } from "../context/me";

dayjs.extend(utc);
dayjs.extend(timezone);
dayjs.extend(relativeTime);

export function useDateTime() {
  const { timezone } = useMe();
  const tz = timezone ?? Intl.DateTimeFormat().resolvedOptions().timeZone;

  const toUtc = (date: Date | string) => {
    if (date instanceof Date) {
      return dayjs.utc(dayjs(date).format("YYYY-MM-DDTHH:mm:ss"));
    }
    return dayjs.utc(date);
  };

  const format = (date: Date | string, fmt = "MMM D, YYYY h:mm A") => {
    return toUtc(date).tz(tz).format(fmt);
  };

  const fromNow = (date: Date | string) => {
    return toUtc(date).tz(tz).fromNow();
  };

  const toDate = (date: Date | string) => toUtc(date).tz(tz).format("MMM D, YYYY");

  const toTime = (date: Date | string) => toUtc(date).tz(tz).format("h:mm A z");

  return { format, fromNow, toDate, toTime };
}
