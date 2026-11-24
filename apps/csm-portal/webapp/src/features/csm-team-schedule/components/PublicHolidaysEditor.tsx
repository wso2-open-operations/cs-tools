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

import { useId, useMemo, useRef, useState, type ChangeEvent, type JSX } from "react";
import { useAddPublicHolidays, usePublicHolidayList, useRemovePublicHoliday } from "../api/usePublicHolidayEditor";
import { holidayFileToLines, MAX_HOLIDAY_FILE_BYTES } from "../utils/holidayFile";
import { parseHolidayLines } from "../utils/holidayLines";

/** What the Import file picker offers: a calendar, a table, or plain text lines. */
const IMPORT_ACCEPT = ".ics,.csv,.tsv,.txt,text/calendar,text/csv,text/tab-separated-values,text/plain";

/** A picked file's text (UTF-8). */
function readText(file: File): Promise<string> {
  return new Promise((resolve, reject) => {
    const reader = new FileReader();
    reader.onload = () => resolve(String(reader.result ?? ""));
    reader.onerror = () => reject(reader.error ?? new Error("read failed"));
    reader.readAsText(file);
  });
}

/** The country picker's entry that lets the reader type another country. */
const OTHER = "__other__";

/** A weekday name for a YYYY-MM-DD date, read as a calendar day (UTC). */
function weekday(date: string): string {
  return new Date(`${date}T00:00:00Z`).toLocaleDateString(undefined, { weekday: "short", timeZone: "UTC" });
}

/**
 * Public holidays, added by hand (Rota Access). Where no calendar connection
 * is configured the holidays come only from here: a portal or rota admin picks
 * a country and pastes its days, one per line. The section renders only for
 * someone the server says may edit (`canEdit`), so it is not offered to leads
 * and Rota Managers, who would be refused. The roster shades the days, and the
 * Public holiday tags (CRE) are placed on the next weekdays' holidays.
 */
export default function PublicHolidaysEditor(): JSX.Element | null {
  const headingId = useId();
  const thisYear = new Date().getFullYear();
  const [year, setYear] = useState(thisYear);
  const list = usePublicHolidayList(`${year}-01-01`, `${year}-12-31`);
  const add = useAddPublicHolidays();
  const remove = useRemovePublicHoliday();

  const [countryChoice, setCountryChoice] = useState<string | null>(null);
  const [otherCountry, setOtherCountry] = useState("");
  const [text, setText] = useState("");
  const fileInput = useRef<HTMLInputElement>(null);
  const [message, setMessage] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  // Whether the server last said this reader may edit. It does not depend on
  // the year, so the section stays (year picker included) while another year
  // loads or fails; only an answer of canEdit: false hides it.
  const [mayEdit, setMayEdit] = useState(false);
  if (list.data && list.data.canEdit !== mayEdit) setMayEdit(list.data.canEdit);

  const countries = list.data?.countries ?? [];
  const choice = countryChoice ?? countries.find((c) => c === "Sri Lanka") ?? countries[0] ?? OTHER;
  const country = choice === OTHER ? otherCountry.trim() : choice;
  const parsed = useMemo(() => parseHolidayLines(text), [text]);
  const stored = useMemo(
    () => (list.data?.holidays ?? []).filter((h) => h.country === country),
    [list.data, country],
  );

  if (!(list.data ? list.data.canEdit : mayEdit)) return null;

  const canAdd = country !== "" && parsed.holidays.length > 0 && parsed.errors.length === 0 && !add.isPending;

  const submit = (): void => {
    setMessage(null);
    setError(null);
    const sent = text;
    add.mutate(
      { country, holidays: parsed.holidays },
      {
        onSuccess: (res) => {
          // Clear the paste that was sent, not one typed while it was being added.
          setText((current) => (current === sent ? "" : current));
          setMessage(`Added ${res.stored} holiday${res.stored === 1 ? "" : "s"} for ${country}.`);
        },
        onError: (e) => setError(e.message || "The holidays were not added. Try again."),
      },
    );
  };

  const removeDay = (date: string): void => {
    setMessage(null);
    setError(null);
    remove.mutate(
      { country, date },
      {
        onSuccess: () => setMessage(`Removed ${date} for ${country}.`),
        onError: (e) => setError(e.message || "That day was not removed. Try again."),
      },
    );
  };

  // Reads a picked file into the paste box (after anything already there), to be
  // checked and added like a paste. Nothing is sent until Add holidays.
  const importFile = async (e: ChangeEvent<HTMLInputElement>): Promise<void> => {
    const file = e.target.files?.[0];
    e.target.value = ""; // the same file can be picked again
    if (!file) return;
    setMessage(null);
    setError(null);
    if (file.size > MAX_HOLIDAY_FILE_BYTES) {
      setError(`${file.name} is larger than 1 MB; a year of holidays is a few KB.`);
      return;
    }
    let content: string;
    try {
      content = await readText(file);
    } catch {
      setError(`${file.name} could not be read.`);
      return;
    }
    const read = holidayFileToLines(file.name, content, year);
    if (!read.text.trim()) {
      setError(`No holidays found in ${file.name}${/\.ics$/i.test(file.name) ? ` for ${year}` : ""}.`);
      return;
    }
    setText((current) => (current.trim() ? `${current.replace(/\s+$/, "")}\n${read.text}` : read.text));
    setMessage(`${read.note} Check the lines, then Add holidays.`);
  };

  return (
    <section className="ra-sec phe" aria-labelledby={headingId}>
      <h3 id={headingId}>Public holidays</h3>
      <p className="cp-help">
        The roster shades each person&apos;s public holidays (their own country&apos;s, else Sri Lanka, or India for
        the India team). Add a country&apos;s holidays here, one per line as <code>YYYY-MM-DD, Name</code>, or import
        a file: a holiday calendar (<code>.ics</code>, the chosen year&apos;s days), a table (<code>.csv</code>, date
        then name) or a text file of such lines. CRE members
        get a Public holiday tag on each weekday holiday, three weeks ahead; their lead removes it for whoever works.
      </p>

      <div className="phe-controls">
        <label className="rrs-field">
          <span>Country</span>
          <select value={choice} onChange={(e) => setCountryChoice(e.target.value)} aria-label="Country">
            {countries.map((c) => (
              <option key={c} value={c}>
                {c}
              </option>
            ))}
            <option value={OTHER}>Another country…</option>
          </select>
        </label>
        {choice === OTHER ? (
          <label className="rrs-field">
            <span>Country name</span>
            <input
              value={otherCountry}
              maxLength={100}
              onChange={(e) => setOtherCountry(e.target.value)}
              placeholder="As people's location spells it"
              aria-label="Country name"
            />
          </label>
        ) : null}
        <label className="rrs-field">
          <span>Year</span>
          <select value={year} onChange={(e) => setYear(Number(e.target.value))} aria-label="Year">
            {[thisYear - 1, thisYear, thisYear + 1, thisYear + 2].map((y) => (
              <option key={y} value={y}>
                {y}
              </option>
            ))}
          </select>
        </label>
      </div>

      {error ? (
        <div className="ra-err" role="alert">
          {error}
        </div>
      ) : null}
      {message ? (
        <p className="rrs-msg" role="status">
          {message}
        </p>
      ) : null}

      {!list.data && list.isError ? (
        <div className="ra-err" role="alert">
          The holidays for {year} could not be loaded.{" "}
          <button type="button" className="btn sm ghost" onClick={() => void list.refetch()}>
            Try again
          </button>
        </div>
      ) : !list.data ? (
        <p className="cp-help" role="status">
          Loading the holidays for {year}…
        </p>
      ) : country ? (
        <table className="ra-tbl phe-tbl" aria-label={`${country} public holidays in ${year}`}>
          <thead>
            <tr>
              <th>Date</th>
              <th>Holiday</th>
              <th>Added</th>
              <th aria-label="Actions" />
            </tr>
          </thead>
          <tbody>
            {stored.length === 0 ? (
              <tr>
                <td colSpan={4} className="cp-help">
                  No holidays stored for {country} in {year}.
                </td>
              </tr>
            ) : (
              stored.map((h) => (
                <tr key={h.date}>
                  <td className="phe-date">
                    {h.date} <span className="phe-wd">{weekday(h.date)}</span>
                  </td>
                  <td>{h.title}</td>
                  <td className="phe-src">{h.source === "MANUAL" ? `By hand${h.updatedBy ? ` (${h.updatedBy})` : ""}` : "From the calendar"}</td>
                  <td>
                    <button
                      type="button"
                      className="btn sm ghost"
                      disabled={remove.isPending}
                      onClick={() => removeDay(h.date)}
                      aria-label={`Remove ${h.title} on ${h.date}`}
                    >
                      Remove
                    </button>
                  </td>
                </tr>
              ))
            )}
          </tbody>
        </table>
      ) : null}

      <label className="rrs-field phe-paste">
        <span>Add holidays for {country || "the country"}</span>
        <textarea
          rows={6}
          value={text}
          onChange={(e) => setText(e.target.value)}
          placeholder={"2027-01-14, Tamil Thai Pongal Day\n2027-02-04, Independence Day"}
          aria-label="Holidays to add, one per line"
        />
      </label>
      <div className="phe-import">
        <input
          ref={fileInput}
          type="file"
          accept={IMPORT_ACCEPT}
          hidden
          onChange={(e) => void importFile(e)}
          aria-label="Holiday file to import"
        />
        <button type="button" className="btn sm ghost" onClick={() => fileInput.current?.click()}>
          Import file…
        </button>
        <span className="cp-help">.ics calendar, .csv table or .txt lines; nothing is added until Add holidays.</span>
      </div>
      {parsed.errors.length > 0 ? (
        <ul className="phe-errs" aria-label="Lines to fix">
          {parsed.errors.map((e) => (
            <li key={`${e.line}-${e.reason}`}>{e.line > 0 ? `Line ${e.line}: ${e.reason}` : e.reason}</li>
          ))}
        </ul>
      ) : null}
      <div className="rrs-acts">
        <span className="cp-help">
          {parsed.holidays.length > 0
            ? `${parsed.holidays.length} holiday${parsed.holidays.length === 1 ? "" : "s"} ready. A day already stored takes the new name.`
            : "Nothing to add yet."}
        </span>
        <button type="button" className="btn sm primary" disabled={!canAdd} onClick={submit}>
          {add.isPending ? "Adding…" : "Add holidays"}
        </button>
      </div>
    </section>
  );
}
