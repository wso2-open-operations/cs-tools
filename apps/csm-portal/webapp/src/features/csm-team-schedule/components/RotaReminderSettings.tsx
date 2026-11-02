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
import { useId, useState, type FormEvent, type JSX } from "react";
import RequiredMark from "@components/RequiredMark";
import { useRotaReminderSettings, useSaveRotaReminderSettings } from "../api/useRotaReminderSettings";
import type { ReminderWindow, RotaFamily, RotaReminderSettings, SaveRotaReminderSettings } from "../types";
import RotaReminderSendNow from "./RotaReminderSendNow";

const FAMILY: RotaFamily = "CRE";

/** The rotation codes this form sets; every other code in the saved settings is kept. */
const LK_WEEKEND = "CRE_WEEKEND";
const AMERICAS_WEEKEND = "CRE_WEEKEND_NIGHT";
const WINDOW_CODES = ["CRE_WEEKEND_NIGHT", "CRE_WEEKEND_NIGHT_OC"] as const;

const MAX_PEOPLE = 20;
const EMAIL = /^[^\s@]+@[^\s@]+\.[^\s@]+$/;

const TIME_ZONES = [
  "America/Los_Angeles",
  "America/New_York",
  "America/Chicago",
  "America/Denver",
  "America/Sao_Paulo",
  "Asia/Colombo",
  "UTC",
];

const DEFAULT_WINDOW: ReminderWindow = { start: "08:00", end: "18:00", timeZone: "America/Los_Angeles" };

/**
 * What the card shows until something is saved. The automatic reminders are
 * off until somebody turns them on here: nothing in the deployment's
 * configuration switches them on.
 */
const DEFAULTS: SaveRotaReminderSettings = {
  weekdayEnabled: false,
  weekendEnabled: false,
  invitesEnabled: true,
  weekendRequired: { [LK_WEEKEND]: 3, [AMERICAS_WEEKEND]: 1 },
  windows: { [WINDOW_CODES[0]]: DEFAULT_WINDOW, [WINDOW_CODES[1]]: DEFAULT_WINDOW },
  copyTo: [],
};

/** The form as typed: numbers and addresses stay text until they are checked. */
interface Form {
  weekdayEnabled: boolean;
  weekendEnabled: boolean;
  invitesEnabled: boolean;
  lkWeekend: string;
  americasWeekend: string;
  start: string;
  end: string;
  timeZone: string;
  copyTo: string;
}

function toForm(s: SaveRotaReminderSettings): Form {
  const required = s.weekendRequired ?? {};
  const windows = s.windows ?? {};
  const w = windows[WINDOW_CODES[0]] ?? windows[WINDOW_CODES[1]] ?? DEFAULT_WINDOW;
  return {
    weekdayEnabled: s.weekdayEnabled,
    weekendEnabled: s.weekendEnabled,
    invitesEnabled: s.invitesEnabled,
    lkWeekend: String(required[LK_WEEKEND] ?? DEFAULTS.weekendRequired[LK_WEEKEND]),
    americasWeekend: String(required[AMERICAS_WEEKEND] ?? DEFAULTS.weekendRequired[AMERICAS_WEEKEND]),
    start: w.start,
    end: w.end,
    timeZone: w.timeZone,
    copyTo: (s.copyTo ?? []).join("\n"),
  };
}

function sameForm(a: Form, b: Form): boolean {
  return (Object.keys(a) as (keyof Form)[]).every((k) => a[k] === b[k]);
}

function wholeNumber(text: string, label: string): number | string {
  const t = text.trim();
  const n = /^\d+$/.test(t) ? Number(t) : NaN;
  if (!Number.isInteger(n) || n < 0 || n > MAX_PEOPLE) {
    return `${label} must be a whole number from 0 to ${MAX_PEOPLE}.`;
  }
  return n;
}

/** The addresses typed, trimmed and without repeats (case aside), or what is wrong with one. */
function addresses(text: string): string[] | string {
  const out: string[] = [];
  const seen = new Set<string>();
  for (const part of text.split(/[,\n\r]+/)) {
    const a = part.trim();
    if (!a) continue;
    if (!EMAIL.test(a)) return `"${a}" is not an email address.`;
    if (seen.has(a.toLowerCase())) continue;
    seen.add(a.toLowerCase());
    out.push(a);
  }
  return out;
}

/**
 * The settings to save: the form checked and laid over what was loaded, so a
 * rotation code this form does not show goes back as it was. An error string
 * says what to fix instead.
 */
/**
 * The body to save. The minimums and the hours are sent for a rotation only
 * when it was already saved or the user changed it here: a rotation left out
 * keeps what the deployment configured (the scheduled task falls back to its
 * own setting for every code the saved settings do not name), so a first save
 * of, say, a copy address never writes the built-in values over the
 * deployment's.
 */
function buildBody(
  form: Form,
  baseForm: Form,
  saved: SaveRotaReminderSettings | null,
): { body: SaveRotaReminderSettings } | { error: string } {
  const lk = wholeNumber(form.lkWeekend, "People needed on LK Weekend each day");
  if (typeof lk === "string") return { error: lk };
  const americas = wholeNumber(form.americasWeekend, "People needed on Americas Weekend each day");
  if (typeof americas === "string") return { error: americas };

  if (!form.start || !form.end) {
    return { error: "Set both the start and the end of the Americas Weekend and LK On-call hours." };
  }
  if (form.start === form.end) {
    return { error: "The start and the end of the Americas Weekend and LK On-call hours must differ." };
  }

  const copyTo = addresses(form.copyTo);
  if (typeof copyTo === "string") return { error: copyTo };

  const savedRequired = saved?.weekendRequired ?? {};
  const savedWindows = saved?.windows ?? {};
  const weekendRequired: Record<string, number> = { ...savedRequired };
  if (LK_WEEKEND in savedRequired || form.lkWeekend !== baseForm.lkWeekend) weekendRequired[LK_WEEKEND] = lk;
  if (AMERICAS_WEEKEND in savedRequired || form.americasWeekend !== baseForm.americasWeekend) {
    weekendRequired[AMERICAS_WEEKEND] = americas;
  }
  const windows: Record<string, ReminderWindow> = { ...savedWindows };
  const hoursChanged =
    form.start !== baseForm.start || form.end !== baseForm.end || form.timeZone !== baseForm.timeZone;
  if (hoursChanged || WINDOW_CODES.some((code) => code in savedWindows)) {
    const hours: ReminderWindow = { start: form.start, end: form.end, timeZone: form.timeZone };
    for (const code of WINDOW_CODES) windows[code] = { ...hours };
  }
  return {
    body: {
      weekdayEnabled: form.weekdayEnabled,
      weekendEnabled: form.weekendEnabled,
      invitesEnabled: form.invitesEnabled,
      weekendRequired,
      windows,
      copyTo,
    },
  };
}

/**
 * The CRE rota reminders card on the Rota Access tab: whether the weekday and
 * weekend reminder emails and the calendar invitations go out, how many people
 * a weekend rotation needs before it is announced, the hours of the Americas
 * Weekend and LK On-call rotations, and who is copied on every reminder.
 *
 * Until something is saved the reminders run on their built-in settings, which
 * the form shows. The server decides who may save (a CSM admin or the CRE rota
 * admin) and answers a refusal in words, shown as it is.
 */
export default function RotaReminderSettings(): JSX.Element {
  const uid = useId();
  const id = (part: string): string => `${uid}-${part}`;
  const headingId = id("h");

  const query = useRotaReminderSettings(FAMILY);
  const save = useSaveRotaReminderSettings(FAMILY);
  const [draft, setDraft] = useState<Form | null>(null);
  const [savedNow, setSavedNow] = useState<RotaReminderSettings | null>(null);
  const [justSaved, setJustSaved] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const fetched = query.data ?? null;
  // What this page saved, until the settings read again are at least as new.
  const loaded = savedNow && (!fetched || fetched.updatedOn < savedNow.updatedOn) ? savedNow : fetched;
  const base = loaded ?? DEFAULTS;
  const baseForm = toForm(base);
  const form = draft ?? baseForm;
  const dirty = draft !== null && !sameForm(draft, baseForm);

  const heading = (
    <h3 id={headingId}>CRE rota reminders</h3>
  );

  if (query.isLoading) {
    return (
      <section className="ra-sec" aria-labelledby={headingId}>
        {heading}
        <p className="cp-help">Loading reminder settings…</p>
      </section>
    );
  }
  if (query.isError) {
    return (
      <section className="ra-sec" aria-labelledby={headingId}>
        {heading}
        <div className="ra-err" role="alert">
          Could not load the reminder settings.
        </div>
      </section>
    );
  }

  const edit = (patch: Partial<Form>): void => {
    setDraft({ ...form, ...patch });
    setJustSaved(false);
    setError(null);
  };

  const discard = (): void => {
    setDraft(null);
    setJustSaved(false);
    setError(null);
  };

  const submit = (e: FormEvent<HTMLFormElement>): void => {
    e.preventDefault();
    if (!dirty || save.isPending) return;
    setJustSaved(false);
    const built = buildBody(form, baseForm, loaded);
    if ("error" in built) {
      setError(built.error);
      return;
    }
    setError(null);
    save.mutate(built.body, {
      onSuccess: (result) => {
        setSavedNow(result);
        setDraft(null);
        setJustSaved(true);
      },
      onError: (err) => setError(err.message || "The reminder settings did not save."),
    });
  };

  // A zone saved by somebody else stays choosable rather than vanishing.
  const zones = TIME_ZONES.includes(form.timeZone) ? TIME_ZONES : [form.timeZone, ...TIME_ZONES];
  const changedOn = loaded?.updatedOn ? new Date(loaded.updatedOn).toLocaleString() : "";

  return (
    <section className="ra-sec" aria-labelledby={headingId}>
      <div className="rrs-head">
        <div className="rrs-title">
          {heading}
          <p className="rrs-intro">Emails and calendar invitations sent to the people on each rotation.</p>
        </div>
        <span className={loaded === null ? "rrs-badge" : "rrs-badge saved"}>
          {loaded === null ? "Built-in settings" : "Saved"}
        </span>
      </div>
      {loaded === null ? (
        <p className="cp-help rrs-meta">Not saved yet: no automatic reminder is sent until one is turned on here and saved. The other values below are the deployment&apos;s defaults; only what you change here is saved.</p>
      ) : (
        <p className="cp-help rrs-meta">
          Last changed by {loaded.updatedBy}
          {changedOn ? ` on ${changedOn}` : ""}
        </p>
      )}

      <div className="rrs-grid2">
        <section className="rrs-panel" aria-labelledby={id("settings-h")}>
          <h4 id={id("settings-h")}>Settings</h4>
          <form className="rrs" noValidate onSubmit={submit}>
            <fieldset className="rrs-group">
              <legend>Automatic reminders</legend>
              <p className="cp-help" id={id("automatic-help")}>
                The only on/off for the automatic reminders in this environment. A change applies from the next run.
              </p>
              <div className="rrs-checks">
                <div>
                  <label className="cas-check" htmlFor={id("weekday")}>
                    <input
                      id={id("weekday")}
                      type="checkbox"
                      checked={form.weekdayEnabled}
                      aria-describedby={id("weekday-help")}
                      onChange={(e) => edit({ weekdayEnabled: e.target.checked })}
                    />
                    Send weekday reminders
                  </label>
                  <p className="cp-help" id={id("weekday-help")}>
                    Morning and Evening, emailed the working day before at 15:00 IST.
                  </p>
                </div>
                <div>
                  <label className="cas-check" htmlFor={id("weekend")}>
                    <input
                      id={id("weekend")}
                      type="checkbox"
                      checked={form.weekendEnabled}
                      aria-describedby={id("weekend-help")}
                      onChange={(e) => edit({ weekendEnabled: e.target.checked })}
                    />
                    Send weekend reminders
                  </label>
                  <p className="cp-help" id={id("weekend-help")}>
                    Sunday to Thursday at 20:00 IST once the weekend is fully marked, and again on Friday at 15:00
                    IST.
                  </p>
                </div>
                <div>
                  <label className="cas-check" htmlFor={id("invites")}>
                    <input
                      id={id("invites")}
                      type="checkbox"
                      checked={form.invitesEnabled}
                      aria-describedby={id("invites-help")}
                      onChange={(e) => edit({ invitesEnabled: e.target.checked })}
                    />
                    Attach calendar invitations
                  </label>
                  <p className="cp-help" id={id("invites-help")}>
                    Everybody on a rotation is a guest of its event, so they can start a group chat from it.
                  </p>
                </div>
              </div>
            </fieldset>

            <fieldset className="rrs-group">
              <legend>Weekend rota</legend>
              <div>
                <div className="rrs-grid">
                  <div className="rrs-field">
                    <label htmlFor={id("lk")}>
                      People needed on LK Weekend each day
                      <RequiredMark />
                    </label>
                    <input
                      id={id("lk")}
                      type="number"
                      required
                      min={0}
                      max={MAX_PEOPLE}
                      step={1}
                      inputMode="numeric"
                      value={form.lkWeekend}
                      aria-describedby={id("people-help")}
                      onChange={(e) => edit({ lkWeekend: e.target.value })}
                    />
                  </div>
                  <div className="rrs-field">
                    <label htmlFor={id("americas")}>
                      People needed on Americas Weekend each day
                      <RequiredMark />
                    </label>
                    <input
                      id={id("americas")}
                      type="number"
                      required
                      min={0}
                      max={MAX_PEOPLE}
                      step={1}
                      inputMode="numeric"
                      value={form.americasWeekend}
                      aria-describedby={id("people-help")}
                      onChange={(e) => edit({ americasWeekend: e.target.value })}
                    />
                  </div>
                </div>
                <p className="cp-help" id={id("people-help")}>
                  Until both are met the weekend is not announced and the CRE rota admins are asked to finish the
                  rota. On-call does not count.
                </p>
              </div>

              <fieldset className="rrs-win" aria-describedby={id("window-help")}>
                <legend>Americas Weekend and LK On-call hours</legend>
                <div className="rrs-times">
                  <div className="rrs-field">
                    <label htmlFor={id("start")}>
                      Start
                      <RequiredMark />
                    </label>
                    <input
                      id={id("start")}
                      type="time"
                      required
                      value={form.start}
                      onChange={(e) => edit({ start: e.target.value })}
                    />
                  </div>
                  <div className="rrs-field">
                    <label htmlFor={id("end")}>
                      End
                      <RequiredMark />
                    </label>
                    <input
                      id={id("end")}
                      type="time"
                      required
                      value={form.end}
                      onChange={(e) => edit({ end: e.target.value })}
                    />
                  </div>
                  <div className="rrs-field">
                    <label htmlFor={id("zone")}>
                      Time zone
                      <RequiredMark />
                    </label>
                    <select
                      id={id("zone")}
                      required
                      value={form.timeZone}
                      onChange={(e) => edit({ timeZone: e.target.value })}
                    >
                      {zones.map((z) => (
                        <option key={z} value={z}>
                          {z}
                        </option>
                      ))}
                    </select>
                  </div>
                </div>
                <p className="cp-help" id={id("window-help")}>
                  Kept on this clock: in IST and UTC the window moves by an hour when US daylight saving changes.
                </p>
              </fieldset>
            </fieldset>

            <fieldset className="rrs-group">
              <legend>Copies</legend>
              <div className="rrs-field">
                <label htmlFor={id("copy")}>Also copy every reminder to</label>
                <textarea
                  id={id("copy")}
                  rows={3}
                  value={form.copyTo}
                  aria-describedby={id("copy-help")}
                  onChange={(e) => edit({ copyTo: e.target.value })}
                />
                <p className="cp-help" id={id("copy-help")}>
                  Email addresses, separated by commas or new lines. Optional.
                </p>
              </div>
            </fieldset>

            <div className="rrs-foot">
              <div className="rrs-msg">
                {error ? (
                  <div className="ra-err" role="alert">
                    {error}
                  </div>
                ) : null}
                <div aria-live="polite">{justSaved ? <p className="cas-status ok">Saved.</p> : null}</div>
              </div>
              <div className="rrs-acts">
                <button type="button" className="btn sm" disabled={!dirty} onClick={discard}>
                  Discard changes
                </button>
                <button type="submit" className="btn sm primary" disabled={!dirty || save.isPending}>
                  {save.isPending ? "Saving…" : "Save reminder settings"}
                </button>
              </div>
            </div>
          </form>
        </section>
        <RotaReminderSendNow />
      </div>
    </section>
  );
}
