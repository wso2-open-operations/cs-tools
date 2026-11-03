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

import { useMemo, useState, type JSX } from "react";
import type { ScheduleActivity } from "../types";
import ScheduleSkeleton from "./ScheduleSkeleton";

interface RecentChangesProps {
  activity: ScheduleActivity[];
  isLoading: boolean;
  /** The history could not be read. Said as such -- an empty list here would
   *  claim nothing had changed. */
  isError?: boolean;
  /** Names for the engineers the rows are about; the history keeps ids. */
  nameOf: (userId: string) => string | undefined;
  /** Labels for a window or leave kind code, as the page already knows them. */
  shiftLabel: (code: string) => string;
  kindLabel: (code: string) => string;
  onClose: () => void;
}

/** One line in the list: a change as a lead would say it. */
interface Entry {
  id: string;
  /** The engineer the change is about, for the person filter. */
  userId: string;
  /** Whether the page knows the engineer's name; an unnamed one reads
   *  "someone" and is left out of the person filter. */
  named: boolean;
  when: Date;
  actor: string;
  who: string;
  days: string;
  what: string;
  subject: "rota" | "leave";
}

/** How many to show before "Show more" -- a month of edits is a long list. */
const PAGE = 25;

/** Two rows written within this of each other are one edit. */
const SAME_EDIT_MS = 10_000;

const fmtDay = (iso: string): string =>
  new Date(`${iso}T00:00:00`).toLocaleDateString(undefined, { weekday: "short", day: "numeric", month: "short" });

function fmtSpan(from: string, to?: string): string {
  if (!to || to === from) return fmtDay(from);
  return `${fmtDay(from)} – ${fmtDay(to)}`;
}

/** The exact moment, in India Standard Time whatever the reader's own zone:
 *  "3 Nov, 09:27 IST". Precise enough to tell a change made before the daily
 *  timesheet report read the roster from one made after. */
const istFormat = new Intl.DateTimeFormat("en-GB", {
  timeZone: "Asia/Colombo",
  day: "numeric",
  month: "short",
  hour: "2-digit",
  minute: "2-digit",
  hour12: false,
});
const fmtIst = (when: Date): string => `${istFormat.format(when)} IST`;

/** "just now", "12 min ago", "3 h ago", then the date. */
function ago(when: Date, now: number): string {
  const s = Math.max(0, Math.round((now - when.getTime()) / 1000));
  if (s < 60) return "just now";
  if (s < 3600) return `${Math.floor(s / 60)} min ago`;
  if (s < 86_400) return `${Math.floor(s / 3600)} h ago`;
  return when.toLocaleDateString(undefined, { day: "numeric", month: "short" });
}

/**
 * The changes a lead's teams have had, newest first: who changed what, for
 * whom, on which days, and when.
 *
 * The history records a changed window as two rows -- the old window deleted,
 * the new one created, in the same write -- so those are paired back into the
 * one edit a lead made ("Regular hours → Evening 6-9pm"). Anything unpaired
 * reads on its own ("set to…", "removed…").
 */
export default function RecentChanges({
  activity,
  isLoading,
  isError = false,
  nameOf,
  shiftLabel,
  kindLabel,
  onClose,
}: RecentChangesProps): JSX.Element {
  const [shown, setShown] = useState(PAGE);
  const [now] = useState(() => Date.now());
  /** The engineer to show changes for; "" is everyone. */
  const [person, setPerson] = useState("");

  const entries = useMemo(() => {
    const out: Entry[] = [];
    const used = new Set<string>();
    const base = (a: ScheduleActivity) => ({
      id: a.id,
      userId: a.userId,
      when: new Date(a.createdOn),
      actor: a.actorEmail.split("@")[0],
      who: nameOf(a.userId) ?? "someone",
      named: nameOf(a.userId) !== undefined,
      subject: a.subject,
    });
    for (const a of activity) {
      if (used.has(a.id)) continue;
      used.add(a.id);
      if (a.subject === "leave") {
        const kind = kindLabel(a.shiftCode);
        const what =
          a.action === "CREATED" ? `marked ${kind}` : a.action === "TRIMMED" ? `${kind} shortened` : `${kind} cleared`;
        out.push({ ...base(a), days: fmtSpan(a.rotaDate, a.endsOn), what });
        continue;
      }
      // An edit in place is its own thing, not half of a replacement: it
      // says which field moved and what it moved between. Reading it as a
      // delete told the panel a slot had been removed when it had only been
      // changed.
      if (a.action === "UPDATED") {
        const field = a.fieldName ?? "";
        const moved =
          a.oldValue && a.newValue ? `${a.oldValue} → ${a.newValue}` : (a.newValue ?? "");
        out.push({
          ...base(a),
          days: fmtDay(a.rotaDate),
          what: moved
            ? `${shiftLabel(a.shiftCode)}: ${field ? `${field} ` : ""}${moved}`
            : `changed ${shiftLabel(a.shiftCode)}`,
        });
        continue;
      }

      // Pair a create with the delete it replaced, for the same person and
      // day. Only those two pair: letting any differing action match meant an
      // edit in place could swallow an unrelated create, and that create then
      // vanished from the panel entirely.
      const t = new Date(a.createdOn).getTime();
      const wanted = a.action === "CREATED" ? "DELETED" : "CREATED";
      const partner = activity.find(
        (b) =>
          !used.has(b.id) &&
          b.subject === "rota" &&
          b.userId === a.userId &&
          b.rotaDate === a.rotaDate &&
          b.action === wanted &&
          Math.abs(new Date(b.createdOn).getTime() - t) <= SAME_EDIT_MS,
      );
      if (partner) used.add(partner.id);
      const created = a.action === "CREATED" ? a : partner?.action === "CREATED" ? partner : undefined;
      const deleted = a.action === "DELETED" ? a : partner?.action === "DELETED" ? partner : undefined;
      const what =
        created && deleted
          ? `${shiftLabel(deleted.shiftCode)} → ${shiftLabel(created.shiftCode)}`
          : created
            ? `set to ${shiftLabel(created.shiftCode)}`
            : `removed ${shiftLabel((deleted ?? a).shiftCode)}`;
      out.push({ ...base(a), days: fmtDay(a.rotaDate), what });
    }
    return out;
  }, [activity, nameOf, shiftLabel, kindLabel]);

  /** Everyone the listed changes are about, by name, for the filter. Only
   *  people the page can name: a list of "someone"s could not be told apart. */
  const people = useMemo(() => {
    const byId = new Map<string, string>();
    for (const e of entries) if (e.named && !byId.has(e.userId)) byId.set(e.userId, e.who);
    return [...byId.entries()].sort((a, b) => a[1].localeCompare(b[1]));
  }, [entries]);
  const visible = person ? entries.filter((e) => e.userId === person) : entries;

  return (
    <section className="recent" aria-label="Recent changes">
      <div className="rc-head">
        <b>Recent changes</b>
        <span className="count">{visible.length}</span>
        <span className="rc-sub">to your teams&rsquo; rota and leave, in the months on screen</span>
        {people.length > 1 || person ? (
          <label className="rc-filter">
            Person
            <select
              value={person}
              onChange={(e) => {
                setPerson(e.target.value);
                setShown(PAGE);
              }}
            >
              <option value="">Everyone</option>
              {people.map(([id, name]) => (
                <option key={id} value={id}>
                  {name}
                </option>
              ))}
            </select>
          </label>
        ) : null}
        <button type="button" className="pk-x" onClick={onClose} aria-label="Close recent changes">
          ×
        </button>
      </div>
      {isLoading ? (
        <ScheduleSkeleton variant="list" label="Loading changes…" rows={3} />
      ) : isError ? (
        <div className="offnone" role="alert">
          Could not load the recent changes. Try again in a moment.
        </div>
      ) : entries.length === 0 ? (
        <div className="offnone">No changes yet in these months.</div>
      ) : visible.length === 0 ? (
        <div className="offnone">No changes to this person in these months.</div>
      ) : (
        <ol className="rc-list">
          {visible.slice(0, shown).map((e) => (
            <li key={e.id} className={`rc-i ${e.subject}`}>
              <span className="rc-when">
                {ago(e.when, now)}
                <time className="rc-at" dateTime={e.when.toISOString()}>
                  {fmtIst(e.when)}
                </time>
              </span>
              <span className="rc-body">
                <b>{e.who}</b> · {e.days}: {e.what}
              </span>
              <span className="rc-by">by {e.actor}</span>
            </li>
          ))}
        </ol>
      )}
      {visible.length > shown ? (
        <button type="button" className="btn sm ghost rc-more" onClick={() => setShown((n) => n + PAGE)}>
          Show {Math.min(PAGE, visible.length - shown)} more
        </button>
      ) : null}
    </section>
  );
}
