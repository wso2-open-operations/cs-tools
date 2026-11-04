/**
 * Copyright (c) 2026 WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

import { useEffect, useState, type JSX } from "react";
import {
  ROSTER_FREEZE_END_MINUTE,
  ROSTER_FREEZE_START_MINUTE,
  fmtIstMinute,
  isRosterFreeze,
  msUntilRosterFreezeChange,
} from "../utils/rosterFreeze";

/** Checked again at least this often, so a sleeping laptop catches up. */
const RECHECK_MS = 60_000;

/**
 * A notice not to edit the roster while the daily timesheet report is being
 * generated from it, shown only from 09:15 to 09:30 IST whatever the reader's
 * own time zone. It appears and disappears on its own: a timer wakes the
 * component at the next edge of the window (and at least once a minute). It
 * only asks; editing still works.
 */
export default function RosterFreezeBanner(): JSX.Element | null {
  const [frozen, setFrozen] = useState(() => isRosterFreeze(new Date()));

  useEffect(() => {
    let timer: ReturnType<typeof setTimeout>;
    const schedule = (): void => {
      const wait = Math.min(msUntilRosterFreezeChange(new Date()) + 250, RECHECK_MS);
      timer = setTimeout(() => {
        setFrozen(isRosterFreeze(new Date()));
        schedule();
      }, wait);
    };
    schedule();
    return () => clearTimeout(timer);
  }, []);

  if (!frozen) return null;
  const from = fmtIstMinute(ROSTER_FREEZE_START_MINUTE);
  const until = fmtIstMinute(ROSTER_FREEZE_END_MINUTE);
  return (
    <div className="freezebar" role="status">
      <b>Don&rsquo;t edit the roster until {until} IST.</b>{" "}
      <span>
        The daily timesheet report is being generated from the roster ({from} &ndash; {until} IST). Changes made now
        may be missing from it.
      </span>
    </div>
  );
}
