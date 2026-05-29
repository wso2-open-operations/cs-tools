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
import { useCallback, useEffect, useState } from "react";

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

  // Ticks every minute to force a re-render, keeping fromNow() output up to date.
  // This causes all consumers of useDateTime to re-render every 60s.
  // NOTE: If this causes performance issues, remove this and extract fromNow
  // into a separate hook (e.g. useFromNow) so only components that need
  // live relative time pay the re-render cost.
  const [, setTick] = useState(0);
  useEffect(() => {
    const id = setInterval(() => setTick((t) => t + 1), 60_000);
    return () => clearInterval(id);
  }, []);

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
