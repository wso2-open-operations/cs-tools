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
import { useId, useState, type JSX } from "react";
import { useReminderPreview, useRotaReminderRequests, useSendRotaReminder } from "../api/useRotaReminderRequests";
import type { RotaFamily, RotaReminderKind, RotaReminderRequest, ScheduleAssignment } from "../types";
import { describeDays, reminderDays } from "../utils/reminderDays";

const FAMILY: RotaFamily = "CRE";

interface Rotation {
  code: string;
  label: string;
}

/** The rotations each reminder is about, in the order they are listed. */
const MORNING: readonly Rotation[] = [
  { code: "CRE_MORNING", label: "Morning" },
  { code: "CRE_MORNING_OC", label: "Morning on-call" },
];
const EVENING: readonly Rotation[] = [{ code: "CRE_EVENING", label: "Evening" }];

const ROTATIONS: Record<RotaReminderKind, readonly Rotation[]> = {
  morning: MORNING,
  evening: EVENING,
  weekday: [...MORNING, ...EVENING],
  weekend: [
    { code: "CRE_WEEKEND", label: "LK Weekend" },
    { code: "CRE_WEEKEND_NIGHT", label: "Americas Weekend" },
    { code: "CRE_WEEKEND_NIGHT_OC", label: "LK On-call" },
  ],
};

const OPTIONS: readonly { kind: RotaReminderKind; label: string }[] = [
  { kind: "morning", label: "Morning" },
  { kind: "evening", label: "Evening" },
  { kind: "weekend", label: "Weekend" },
];

const KIND_LABEL: Record<RotaReminderKind, string> = {
  morning: "Morning",
  evening: "Evening",
  weekday: "Weekday",
  weekend: "Weekend",
};

/**
 * Whether a waiting request of kind `queued` already sends what `asked` would:
 * the same kind, or an older "weekday" request, which sends the Morning and
 * the Evening together.
 */
function covers(queued: RotaReminderKind, asked: RotaReminderKind): boolean {
  return queued === asked || (queued === "weekday" && (asked === "morning" || asked === "evening"));
}

/** What the last ask said, once it was taken: a real one, or a test copy. */
type Notice = "queued" | "test";

interface PreviewDay {
  date: string;
  rotations: { label: string; names: string[] }[];
}

/** One person, whoever lists them: by email, or by user id when there is none. */
function personKey(a: ScheduleAssignment): string {
  return (a.engineer.email || a.engineer.userId).toLowerCase();
}

/**
 * Who is on each rotation on each day, and how many different people that is
 * in all (one person on two rotations is one email). Assignments of any other
 * rotation are not what the reminder is about and are left out.
 */
function buildPreview(
  kind: RotaReminderKind,
  days: readonly string[],
  assignments: readonly ScheduleAssignment[],
): { days: PreviewDay[]; people: number } {
  const everyone = new Set<string>();
  const perDay = days.map((date): PreviewDay => ({
    date,
    rotations: ROTATIONS[kind].map(({ code, label }) => {
      const names = new Map<string, string>();
      for (const a of assignments) {
        if (a.shiftCode !== code || a.rotaDate.slice(0, 10) !== date) continue;
        const key = personKey(a);
        everyone.add(key);
        if (!names.has(key)) names.set(key, a.engineer.name || a.engineer.email);
      }
      return { label, names: [...names.values()].sort((x, y) => x.localeCompare(y)) };
    }),
  }));
  return { days: perDay, people: everyone.size };
}

/** The people on a rotation as "3 · Ann, Ben, Cid", or "nobody". */
function peopleCell(names: readonly string[]): string {
  return names.length === 0 ? "nobody" : `${names.length} · ${names.join(", ")}`;
}

function when(iso: string): string {
  return new Date(iso).toLocaleString();
}

function whatCell(r: RotaReminderRequest): string {
  return `${r.test ? "Test · " : ""}${KIND_LABEL[r.kind]}`;
}

function statusCell(r: RotaReminderRequest): string {
  switch (r.status) {
    case "QUEUED":
      return "Waiting to be sent";
    case "SENDING":
      return "Sending…";
    case "SENT":
      return `Sent${r.handledOn ? ` on ${when(r.handledOn)}` : ""}${r.result ? `: ${r.result}` : ""}`;
    case "FAILED":
      return `Failed${r.result ? `: ${r.result}` : ""}`;
  }
}

/**
 * The "Send now" panel of the CRE rota reminders card: asks for the next working
 * day's Morning or Evening reminder, or the weekend's, and their calendar
 * invitations, to go out now rather
 * than at their usual time. It shows who would be emailed first and asks once
 * more before it asks. The ask is queued: the scheduled task sends it at its
 * next run. "Send a test to me" asks for the same reminder to reach only the
 * person asking, with no confirmation, since it cannot reach anybody else. The
 * server decides who may ask and how often, and answers a refusal in words,
 * shown as it is.
 */
export default function RotaReminderSendNow(): JSX.Element {
  const uid = useId();
  const id = (part: string): string => `${uid}-${part}`;
  const headingId = id("h");

  const [kind, setKind] = useState<RotaReminderKind>("morning");
  const [confirming, setConfirming] = useState(false);
  const [notice, setNotice] = useState<Notice | null>(null);
  const [error, setError] = useState<string | null>(null);

  const days = reminderDays(kind, new Date());
  const from = days[0] ?? "";
  const to = days[days.length - 1] ?? from;

  const requests = useRotaReminderRequests(FAMILY);
  const send = useSendRotaReminder(FAMILY);
  const roster = useReminderPreview(from, to, true);

  const recent = requests.data ?? [];
  // A test copy reaches only the person who asked and changes nothing real, so
  // a waiting one does not hold the real reminder back.
  const waiting = recent.some(
    (r) => !r.test && covers(r.kind, kind) && (r.status === "QUEUED" || r.status === "SENDING"),
  );

  const preview = roster.data ? buildPreview(kind, days, roster.data.assignments) : null;
  const nobody = preview !== null && preview.people === 0;
  const canAsk = preview !== null && !nobody && !waiting && !send.isPending;
  const whenText = describeDays(days);

  const pick = (next: RotaReminderKind): void => {
    setKind(next);
    setConfirming(false);
    setNotice(null);
    setError(null);
  };

  const ask = (): void => {
    if (!canAsk) return;
    setError(null);
    setNotice(null);
    send.mutate(
      { kind, test: false },
      {
        onSuccess: () => {
          setConfirming(false);
          setNotice("queued");
        },
        onError: (err) => {
          setConfirming(false);
          setError(err.message || "The reminders were not requested.");
        },
      },
    );
  };

  const askTest = (): void => {
    if (send.isPending) return;
    setError(null);
    setNotice(null);
    send.mutate(
      { kind, test: true },
      {
        onSuccess: () => setNotice("test"),
        onError: (err) => setError(err.message || "The test was not requested."),
      },
    );
  };

  return (
    <section className="rrs-panel rrs-send" aria-labelledby={headingId}>
      <h4 id={headingId}>Send now</h4>
      <p className="cp-help">
        Sends the reminders and calendar invitations now, whatever the on/off switches in Settings say. They go out at
        the scheduled task&apos;s next run, within the hour.
      </p>

      <fieldset className="rrs-kinds">
        <legend>What to send</legend>
        <div className="seg rrs-seg">
          {OPTIONS.map((o) => (
            <label key={o.kind} className={kind === o.kind ? "on" : undefined}>
              <input
                type="radio"
                name={id("kind")}
                value={o.kind}
                checked={kind === o.kind}
                aria-describedby={kind === o.kind ? id("when") : undefined}
                onChange={() => pick(o.kind)}
              />
              {o.label}
            </label>
          ))}
        </div>
        <p className="rrs-when" id={id("when")}>
          {whenText}
        </p>
        {kind === "weekend" ? (
          <p className="cp-help">
            Sent even if the weekend is not fully marked: what is missing is listed and the CRE rota admins are told.
            Sending again updates the same calendar events.
          </p>
        ) : null}
      </fieldset>

      {roster.isLoading ? <p className="cp-help">Loading who is on the rota…</p> : null}
      {roster.isError ? (
        <div className="ra-err" role="alert">
          Could not load the rota for the preview.
        </div>
      ) : null}
      {preview ? (
        <div className="rrs-tblwrap">
          <table className="rrs-preview" aria-labelledby={id("who")}>
            <caption id={id("who")}>Who gets it</caption>
            <thead>
              <tr>
                <th scope="col">Day</th>
                <th scope="col">Rotation</th>
                <th scope="col">People</th>
              </tr>
            </thead>
            <tbody>
              {preview.days.flatMap((d) =>
                d.rotations.map((r) => (
                  <tr key={`${d.date}-${r.label}`}>
                    <td>{describeDays([d.date])}</td>
                    <td>{r.label}</td>
                    <td>{peopleCell(r.names)}</td>
                  </tr>
                )),
              )}
            </tbody>
          </table>
          <p className="cp-help">People on leave are left out.</p>
        </div>
      ) : null}
      {nobody ? (
        <p className="cas-status info">Nobody is on these rotations, so nothing would be sent.</p>
      ) : null}

      {waiting ? (
        <p className="cp-help">A {KIND_LABEL[kind].toLowerCase()} reminder is already waiting to be sent.</p>
      ) : null}

      {confirming ? (
        <div className="rrs-acts" role="group" aria-labelledby={id("confirm")}>
          <span id={id("confirm")}>{(preview?.people ?? 0) === 1 ? "Email 1 person now?" : `Email ${preview?.people ?? 0} people now?`}</span>
          <button type="button" className="btn sm primary" disabled={!canAsk} onClick={ask}>
            Yes, send
          </button>
          <button type="button" className="btn sm" disabled={send.isPending} onClick={() => setConfirming(false)}>
            Cancel
          </button>
        </div>
      ) : (
        <div className="rrs-acts">
          <button type="button" className="btn sm" disabled={send.isPending} onClick={askTest}>
            Send a test to me
          </button>
          <button
            type="button"
            className="btn sm primary"
            disabled={!canAsk}
            onClick={() => {
              setNotice(null);
              setError(null);
              setConfirming(true);
            }}
          >
            Send now
          </button>
        </div>
      )}

      {error ? (
        <div className="ra-err" role="alert">
          {error}
        </div>
      ) : null}
      <div aria-live="polite">
        {notice === "queued" ? (
          <p className="cas-status ok">Queued. It goes out at the scheduled task&apos;s next run.</p>
        ) : null}
        {notice === "test" ? (
          <p className="cas-status ok">Test queued: it goes only to you, at the scheduled task&apos;s next run.</p>
        ) : null}
      </div>

      {recent.length > 0 ? (
        <div className="rrs-tblwrap">
          <h5 id={id("recent")}>Recent</h5>
          <table className="rrs-recent" aria-labelledby={id("recent")}>
            <thead>
              <tr>
                <th scope="col">When</th>
                <th scope="col">What</th>
                <th scope="col">By</th>
                <th scope="col">Status</th>
              </tr>
            </thead>
            <tbody>
              {recent.map((r) => (
                <tr key={r.id}>
                  <td>{when(r.requestedOn)}</td>
                  <td>{whatCell(r)}</td>
                  <td>{r.requestedBy}</td>
                  <td>{statusCell(r)}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      ) : null}
    </section>
  );
}
