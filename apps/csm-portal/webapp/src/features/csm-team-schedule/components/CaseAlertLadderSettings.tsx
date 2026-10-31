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

import { useId, useRef, useState, type FormEvent, type JSX, type ReactNode, type RefObject } from "react";
import RequiredMark from "@components/RequiredMark";
import { familyOfLadder } from "../utils/caseAlertChainSettings";
import { useCaseAlertDrafts, type DraftChange } from "../utils/caseAlertDrafts";
import type {
  CaseAlertFileOnly,
  CaseAlertLadderChannels,
  CaseAlertRoomLadder,
  CaseAlertLadderValue,
  CaseAlertRunningValue,
  CaseAlertSettings as Settings,
} from "../types";
import {
  ADVANCED_GROUPS,
  CRE_STEP_LABELS,
  channelWords,
  creTimingRows,
  draftFor,
  fileOnlyRows,
  formatLadderValue,
  ladderKeysByName,
  ladderKeysFor,
  ladderValueRequired,
  limitHint,
  parseDraft,
  sameLadderValue,
  teamEmailErrors,
  teamEmailRows,
  teamEmailRowsFor,
  type LadderDraft,
  type LadderKeyGroup,
  type LadderKeySpec,
  type TeamEmailRow,
} from "../utils/caseAlertLadderKeys";
import CaseAlertModal from "./CaseAlertModal";

function when(iso: string | undefined): string {
  return iso ? new Date(iso).toLocaleString() : "";
}

/** The saved keys of a ladder this page doesn't know. */
function unknownSaved(s: Settings, ladder: CaseAlertRoomLadder): CaseAlertLadderValue[] {
  const known = new Set(ladderKeysFor(ladder).map((k) => k.key));
  return s.values.filter((v) => v.ladder === ladder && !known.has(v.key));
}

interface GroupProps {
  ladder: CaseAlertRoomLadder;
  settings: Settings;
  group: LadderKeyGroup;
  /** Shown above the table, e.g. the ladder's name when a family has two. */
  heading?: string;
  /** Says what was saved, for the settings' confirmation line. */
  onSaved: (what: string) => void;
}

/**
 * One group of a ladder's paging file, key by key. Each row shows the value
 * the alert service runs with and where it came from, and, when one is saved
 * here that isn't in use yet, the saved one. A saved value replaces that one
 * key of the file; going back to the file's removes it. Changes are made in a
 * dialog from the row's Edit. Who may change a ladder's values is who may
 * change its rooms (canManage.roomLadders); everyone else reads it. Nothing
 * when the ladder has no key in the group.
 */
export function LadderValueGroup({ ladder, settings: s, group, heading, onSaved }: GroupProps): JSX.Element | null {
  return (
    <ValueRows
      ladder={ladder}
      settings={s}
      rows={ladderKeysFor(ladder).filter((k) => k.group === group)}
      heading={heading}
      onSaved={onSaved}
    />
  );
}

interface KeysProps extends Omit<GroupProps, "group"> {
  /** The keys to show, in this order. */
  keys: readonly string[];
}

/** A ladder's values for these keys, in one table, as LadderValueGroup shows a
 *  group's. */
export function LadderValueKeys({ ladder, settings: s, keys, heading, onSaved }: KeysProps): JSX.Element | null {
  return (
    <ValueRows ladder={ladder} settings={s} rows={ladderKeysByName(ladder, keys)} heading={heading} onSaved={onSaved} />
  );
}

function ValueRows({
  ladder,
  settings: s,
  rows,
  heading,
  onSaved,
}: Omit<GroupProps, "group"> & { rows: LadderKeySpec[] }): JSX.Element | null {
  if (rows.length === 0) return null;
  const canEdit = s.canManage.roomLadders.includes(ladder);
  const running = s.running?.ladders[ladder] ?? {};
  return (
    <div className="cas-block">
      {heading ? <h5>{heading}</h5> : null}
      <ValueTable canEdit={canEdit}>
        {rows.map((spec) => (
          <ValueRow
            key={spec.key}
            ladder={ladder}
            spec={spec}
            saved={s.values.find((v) => v.ladder === ladder && v.key === spec.key)}
            running={running[spec.key]}
            reported={Boolean(s.running)}
            canEdit={canEdit}
            teamRows={spec.editor === "teamEmails" ? teamEmailRows(s, ladder) : undefined}
            onSaved={onSaved}
          />
        ))}
      </ValueTable>
    </div>
  );
}

interface AdvancedProps {
  ladder: CaseAlertRoomLadder;
  settings: Settings;
  /** The ladder's step of copying the file's values here, when it has one. */
  setup?: ReactNode;
  onSaved: (what: string) => void;
}

/**
 * The rest of a ladder, under the family's Advanced: copying the file's values
 * here, when a ladder starts, how it climbs, the heads, the chat messages, any
 * saved key this page doesn't know, who gets which channel as running, and
 * what only the file sets.
 */
export function LadderAdvanced({ ladder, settings: s, setup, onSaved }: AdvancedProps): JSX.Element {
  const canEdit = s.canManage.roomLadders.includes(ladder);
  const running = s.running?.ladders[ladder] ?? {};
  // A saved key this page doesn't know is still shown, so nothing saved is
  // hidden, and can be taken back to the file's.
  const others = unknownSaved(s, ladder);
  return (
    <div className="cas-ladder" role="group" aria-label={`${ladder} ladder`}>
      <h4>{ladder} ladder</h4>
      {setup}
      {ADVANCED_GROUPS.map((group) => (
        <LadderValueGroup key={group} ladder={ladder} settings={s} group={group} heading={group} onSaved={onSaved} />
      ))}
      {others.length > 0 ? (
        <div className="cas-block">
          <h5>Other saved settings</h5>
          <ValueTable canEdit={canEdit}>
            {others.map((v) => (
              <ValueRow
                key={v.id}
                ladder={ladder}
                spec={{
                  key: v.key,
                  ladders: [ladder],
                  group: "How it climbs",
                  label: v.key,
                  help: "Not a setting this page knows. Go back to the file's value to remove it.",
                  editor: "text",
                }}
                saved={v}
                running={running[v.key]}
                reported={Boolean(s.running)}
                canEdit={canEdit}
                unknown
                onSaved={onSaved}
              />
            ))}
          </ValueTable>
        </div>
      ) : null}
      <ChannelsGroup ladder={ladder} channels={s.running?.channels?.[ladder]} />
      <FileOnlyGroup
        ladder={ladder}
        fileOnly={s.running?.fileOnly?.[ladder]}
        onTab={new Set(Object.keys(running))}
        aliases={ladder === "CRE" ? s.running?.groups?.CRE?.aliases : undefined}
      />
    </div>
  );
}

/** Who gets which channel, as the alert service runs it now: whether the
 *  ladder runs, its channel, and each team's (the ladder's narrowed by the
 *  team's own). Read-only: the ladder's switches are edited under On/off and
 *  channel, a team's channel and room under Where alerts go. */
function ChannelsGroup({
  ladder,
  channels,
}: {
  ladder: CaseAlertRoomLadder;
  channels?: CaseAlertLadderChannels;
}): JSX.Element | null {
  if (!channels) return null;
  return (
    <div className="cas-block" role="group" aria-label={`${ladder}: who gets which channel`}>
      <h5>Who gets which channel</h5>
      <p className="cas-sub">
        {channels.enabled
          ? `The ${ladder} ladder is on: ${channelWords(channels.channel) ?? channels.channel}.`
          : `The ${ladder} ladder is off: nobody is alerted.`}{" "}
        A team's own channel can only narrow the ladder's.
      </p>
      {channels.teams.length > 0 ? (
        <table className="ra-tbl cas-values cas-channels">
          <thead>
            <tr>
              <th>Team</th>
              <th>Channel in use</th>
              <th>Own room</th>
            </tr>
          </thead>
          <tbody>
            {channels.teams.map((t) => (
              <tr key={t.team}>
                <td>
                  <b>{t.team}</b>
                </td>
                <td>
                  <span className="cas-val">
                    {channels.enabled ? (channelWords(t.channel) ?? t.channel) : "Off"}
                    {t.teamChannel ? " (team's own)" : ""}
                  </span>
                </td>
                <td>{t.room ? "Yes" : "No (shared room)"}</td>
              </tr>
            ))}
          </tbody>
        </table>
      ) : (
        <p className="cas-sub cas-empty">Every team uses the ladder's channel.</p>
      )}
    </div>
  );
}

/** The settings only the alert service's file can set, read-only for
 *  everyone. Whether a ladder runs and its channel are edited under On/off
 *  and channel, and its call limits under Advanced, once the alert service
 *  reports them; the rest stay a reviewed change to the file. */
function FileOnlyGroup({
  ladder,
  fileOnly,
  onTab,
  aliases,
}: {
  ladder: CaseAlertRoomLadder;
  fileOnly?: CaseAlertFileOnly;
  onTab: ReadonlySet<string>;
  /** CRE: case team names the file maps to a team key. */
  aliases?: Record<string, string>;
}): JSX.Element {
  const aliasRows = Object.entries(aliases ?? {}).sort(([a], [b]) => a.localeCompare(b));
  return (
    <div className="cas-block" role="group" aria-label={`${ladder}: set in the alert service's file`}>
      <h5>Set in the alert service&apos;s file (read-only)</h5>
      <p className="cas-sub">
        These decide what alerts may cost. They change only with a reviewed change to the file.
      </p>
      {fileOnly ? (
        <table className="ra-tbl cas-values cas-file-only">
          <colgroup>
            <col className="cas-col-setting" />
            <col />
          </colgroup>
          <thead>
            <tr>
              <th>Setting</th>
              <th>In use</th>
            </tr>
          </thead>
          <tbody>
            {fileOnlyRows(ladder, fileOnly, onTab).map((r) => (
              <tr key={r.label}>
                <td>
                  <b>{r.label}</b>
                </td>
                <td>
                  <span className="cas-val">{r.value}</span>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      ) : (
        <p className="cas-sub cas-empty">
          Not reported yet: the alert service reports these when it runs in shadow or db mode.
        </p>
      )}
      {ladder === "CRE" ? <CreTimingTable fileOnly={fileOnly} /> : null}
      {aliasRows.length > 0 ? (
        <>
          <h5>Team names mapped in the file</h5>
          <p className="cas-sub">A case whose team name is on the left is routed as the team on the right.</p>
          <table className="ra-tbl cas-values cas-aliases" aria-label="CRE: team names mapped in the file">
            <thead>
              <tr>
                <th>Case team name</th>
                <th>Routed as team</th>
              </tr>
            </thead>
            <tbody>
              {aliasRows.map(([from, to]) => (
                <tr key={from}>
                  <td>{from}</td>
                  <td>
                    <span className="cas-val">{to}</span>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </>
      ) : null}
    </div>
  );
}

/** When each CRE step is called, per severity, as the alert service reports
 *  it (minutes after the case is created). Nothing until it has reported. */
export function CreTimingTable({
  fileOnly,
  heading = "When each step is called",
}: {
  fileOnly?: CaseAlertFileOnly;
  heading?: string;
}): JSX.Element | null {
  const timing = creTimingRows(fileOnly?.timing);
  if (timing.length === 0) return null;
  return (
    <>
      {heading ? <h5>{heading}</h5> : null}
      <p className="cas-sub">
        Minutes after the case is created. Each number is one call to that step&apos;s people; the chain moves up only
        while nobody has acknowledged. Set in the code: a change needs a release.
      </p>
      <table className="ra-tbl cas-values cas-timing" aria-label="CRE: when each step is called">
        <thead>
          <tr>
            <th>Severity</th>
            {CRE_STEP_LABELS.map((l) => (
              <th key={l}>{l}</th>
            ))}
            <th>Chain ends</th>
          </tr>
        </thead>
        <tbody>
          {timing.map((row) => (
            <tr key={row.severity}>
              <td>
                <b>{row.severity}</b>
              </td>
              {row.steps.map((v, i) => (
                <td key={CRE_STEP_LABELS[i]}>
                  <span className="cas-val">{v}</span>
                </td>
              ))}
              <td>
                <span className="cas-val">{row.ends}</span>
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </>
  );
}

/** One group's rows. Every group is its own table, so the widths are fixed
 *  here (cas-col-*) and the columns line up from one group to the next. */
function ValueTable({ canEdit, children }: { canEdit: boolean; children: JSX.Element[] }): JSX.Element {
  return (
    <table className="ra-tbl cas-values">
      <colgroup>
        <col className="cas-col-setting" />
        <col />
        {/* Kept when it stays empty, so a read-only page lines up the same. */}
        <col className="cas-col-actions" />
      </colgroup>
      <thead>
        <tr>
          <th>Setting</th>
          <th>In use</th>
          {canEdit ? <th aria-label="Actions" /> : null}
        </tr>
      </thead>
      <tbody>{children}</tbody>
    </table>
  );
}

interface ValueRowProps {
  ladder: CaseAlertRoomLadder;
  spec: LadderKeySpec;
  saved?: CaseAlertLadderValue;
  running?: CaseAlertRunningValue;
  /** Whether the alert service has reported at all. */
  reported: boolean;
  canEdit: boolean;
  /** A key this page doesn't know: it can only go back to the file's. */
  unknown?: boolean;
  /** The teams a team email list has a field for. */
  teamRows?: TeamEmailRow[];
  onSaved: (what: string) => void;
}

/** A team email list in the row: each team that has a list, by name, and
 *  how many teams have one. */
function TeamEmailList({ rows, value, empty }: { rows: TeamEmailRow[]; value: unknown; empty: string }): JSX.Element {
  const map = value && typeof value === "object" && !Array.isArray(value) ? (value as Record<string, unknown>) : {};
  const set = teamEmailRowsFor(rows, map).filter((r) => typeof map[r.key] === "string" && String(map[r.key]).trim() !== "");
  if (set.length === 0) return <span className="cas-val">{empty}</span>;
  // A team that is no longer offered is listed, marked, but not counted.
  const used = set.filter((r) => !r.stale).length;
  const ofTeams = rows.length > 0 ? ` of ${rows.length} teams` : used === 1 ? " team" : " teams";
  return (
    <div className="cas-val">
      <ul className="cas-team-list">
        {set.map((r) => (
          <li key={r.key} className={r.stale ? "cas-stale" : undefined}>
            <span className="cas-team-name">{r.name}</span> <span className="cas-team-addr">{String(map[r.key])}</span>
            {r.stale ? <span className="cas-sub"> (not used)</span> : null}
          </li>
        ))}
      </ul>
      <div className="cas-sub">
        {used}
        {ofTeams} {used === 1 ? "has" : "have"} a list
      </div>
    </div>
  );
}

function ValueRow({
  ladder,
  spec,
  saved,
  running,
  reported,
  canEdit,
  unknown,
  teamRows,
  onSaved,
}: ValueRowProps): JSX.Element {
  const [editing, setEditing] = useState(false);
  const editRef = useRef<HTMLButtonElement>(null);
  const drafts = useCaseAlertDrafts();
  const family = familyOfLadder(ladder);
  // The saved value, while it isn't the one the alert service runs with.
  const pending = saved && !(running && sameLadderValue(spec, saved.value, running.value));
  // A yes or no is a switch, a pick from a few a dropdown, right in the row;
  // the rest open a dialog. Either way the change waits for Submit.
  const inline = canEdit && !unknown && (spec.editor === "bool" || spec.editor === "enum");
  const queued = drafts.changes.find(
    (c): c is Extract<DraftChange, { kind: "values" }> =>
      c.kind === "values" && c.input.ladder === ladder && spec.key in c.input.values,
  );
  const current = queued ? queued.input.values[spec.key] : saved ? saved.value : running?.value;
  // Going back to the file's value is queued: say so, and don't offer it twice.
  const resetQueued = Boolean(saved && drafts.changes.some((c) => c.kind === "deleteValue" && c.valueId === saved.id));
  const change = (value: unknown) => {
    const what = `${ladder} ${spec.label}`;
    drafts.add({ family, what, kind: "values", input: { ladder, values: { [spec.key]: value } } });
    onSaved(what);
  };
  const useFile = () => {
    if (!saved) return;
    const what = `${ladder} ${spec.label} uses the file's value again`;
    drafts.add({ family, what, kind: "deleteValue", valueId: saved.id });
    onSaved(what);
  };

  return (
    <tr>
      <td>
        <b>{spec.label}</b>
        <div className="cas-sub">{spec.help}</div>
      </td>
      <td>
        {inline ? (
          <span className="cas-inline">
            {spec.editor === "bool" ? (
              <button
                type="button"
                role="switch"
                aria-checked={current === true}
                aria-label={spec.label}
                className={`switch${current === true ? " on" : ""}`}
                disabled={drafts.submitting === family}
                onClick={() => change(current !== true)}
              >
                <span className="tr" aria-hidden="true" />
                {current === true ? "On" : "Off"}
              </button>
            ) : (
              <select
                className="cas-select"
                aria-label={spec.label}
                value={typeof current === "string" ? current : ""}
                disabled={drafts.submitting === family}
                onChange={(e) => change(e.target.value)}
              >
                {typeof current === "string" ? null : <option value="">Not reported yet</option>}
                {(spec.options ?? []).map((o) => (
                  <option key={o.value} value={o.value}>
                    {o.label}
                  </option>
                ))}
              </select>
            )}
            {queued ? (
              <span className="cas-chip cas-src-pending">waits for Submit</span>
            ) : resetQueued ? (
              <span className="cas-chip cas-src-pending">file&apos;s value on Submit</span>
            ) : pending && running ? (
              // The control shows the saved value; say what still runs.
              <span className={`cas-chip cas-src-${running.source}`}>
                In use: {formatLadderValue(spec, running.value)}
              </span>
            ) : running ? (
              <span className={`cas-chip cas-src-${running.source}`}>
                {running.source === "file" ? "from the file" : "saved here"}
              </span>
            ) : reported ? null : (
              <span className="cas-chip">not reported yet</span>
            )}
          </span>
        ) : queued || resetQueued ? (
          // A change from the dialog waits for Submit: show it, as a switch does.
          <>
            {queued && teamRows ? (
              <TeamEmailList rows={teamRows} value={current} empty={spec.empty ?? "none"} />
            ) : queued ? (
              <span className="cas-val">{formatLadderValue(spec, current)}</span>
            ) : null}
            <span className="cas-chip cas-src-pending">{queued ? "waits for Submit" : "file's value on Submit"}</span>
          </>
        ) : running ? (
          <>
            {teamRows ? (
              <TeamEmailList rows={teamRows} value={running.value} empty={spec.empty ?? "none"} />
            ) : (
              <span className="cas-val">{formatLadderValue(spec, running.value)}</span>
            )}
            <span className={`cas-chip cas-src-${running.source}`}>
              {running.source === "file" ? "from the file" : "saved here"}
            </span>
          </>
        ) : (
          <>
            <span className="cas-sub">—</span>
            {reported ? null : <span className="cas-chip">not reported yet</span>}
          </>
        )}
        {pending && !queued ? (
          <div className="cas-sub cas-saved" title={`Saved by ${saved.updatedBy} at ${when(saved.updatedOn)}`}>
            {teamRows ? (
              <>
                Saved, not in use yet:
                <TeamEmailList rows={teamRows} value={saved.value} empty={spec.empty ?? "none"} />
              </>
            ) : (
              <>Saved: {formatLadderValue(spec, saved.value)} · not in use yet</>
            )}
          </div>
        ) : null}
      </td>
      {canEdit && inline ? (
        <td>
          {saved && !resetQueued ? (
            <div className="cas-acts">
              <button type="button" className="btn sm" onClick={useFile} aria-label={`${spec.label}: use the file's value`}>
                Use file&apos;s value
              </button>
            </div>
          ) : null}
        </td>
      ) : canEdit ? (
        <td>
          <div className="cas-acts">
            <button ref={editRef} type="button" className="btn sm" onClick={() => setEditing(true)}>
              Edit
            </button>
          </div>
          {editing ? (
            <ValueDialog
              ladder={ladder}
              spec={spec}
              saved={saved}
              start={saved ? saved.value : running?.value}
              unknown={unknown}
              teamRows={teamRows}
              returnFocus={editRef}
              onClose={() => setEditing(false)}
              onSaved={onSaved}
            />
          ) : null}
        </td>
      ) : null}
    </tr>
  );
}

interface ValueDialogProps {
  ladder: CaseAlertRoomLadder;
  spec: LadderKeySpec;
  saved?: CaseAlertLadderValue;
  /** What the editor starts from: the saved value, else the one in use. */
  start: unknown;
  unknown?: boolean;
  teamRows?: TeamEmailRow[];
  /** The Edit button, which gets focus back when the dialog closes. */
  returnFocus: RefObject<HTMLButtonElement | null>;
  onClose: () => void;
  /** Says what was saved, for the confirmation line. */
  onSaved: (what: string) => void;
}

/** Change one ladder value, or go back to the file's, in the Alert Settings
 *  modal (CaseAlertModal). Focus goes back to the row's Edit. */
function ValueDialog({
  ladder,
  spec,
  saved,
  start,
  unknown,
  teamRows,
  returnFocus,
  onClose,
  onSaved,
}: ValueDialogProps): JSX.Element {
  const drafts = useCaseAlertDrafts();
  const family = familyOfLadder(ladder);
  const ids = useId();
  const titleId = `${ids}-title`;
  const helpId = `${ids}-help`;
  const [draft, setDraft] = useState<LadderDraft>(() => draftFor(spec, start));
  const [error, setError] = useState<string | null>(null);
  const [confirmFile, setConfirmFile] = useState(false);
  // Each team's field is marked once a send was tried, and clears as it is fixed.
  const [tried, setTried] = useState(false);
  const busy = drafts.submitting === family;
  const hint = unknown ? null : limitHint(spec);
  const rows = teamRows ? teamEmailRowsFor(teamRows, start) : undefined;
  const rowErrors = tried && rows ? teamEmailErrors(draft) : {};

  // min and max are hints: the server checks every limit and its message
  // names the key, so the browser doesn't stop the send.
  const submit = (e: FormEvent) => {
    e.preventDefault();
    if (busy || unknown) return;
    setError(null);
    setTried(true);
    const parsed = parseDraft(spec, draft);
    if (!parsed.ok) {
      setError(parsed.error);
      return;
    }
    const what = `${ladder} ${spec.label}`;
    drafts.add({ family, what, kind: "values", input: { ladder, values: { [spec.key]: parsed.value } } });
    onSaved(what);
    onClose();
  };

  const goBackToFile = () => {
    if (!saved || busy) return;
    setError(null);
    const what = `${ladder} ${spec.label} uses the file's value again`;
    drafts.add({ family, what, kind: "deleteValue", valueId: saved.id });
    onSaved(what);
    onClose();
  };

  return (
    <CaseAlertModal titleId={titleId} describedBy={helpId} returnFocus={returnFocus} onClose={onClose}>
      <form className="cas-edit" noValidate onSubmit={submit}>
        <h3 id={titleId}>{spec.label}</h3>
        <p id={helpId} className="cp-help">
          {spec.help}
          {hint ? ` ${hint}` : ""}
        </p>
        {unknown ? (
          <p className="cp-help">Saved here: {formatLadderValue(undefined, saved?.value)}</p>
        ) : (
          <Editor spec={spec} draft={draft} onChange={setDraft} teamRows={rows} errors={rowErrors} />
        )}
        {error ? (
          <p className="cp-err" role="alert">
            {error}
          </p>
        ) : null}
        {saved ? (
          <div className="cas-acts cas-dlg-file">
            {confirmFile ? (
              <>
                <span className="cas-sub">The saved value is removed and the file&apos;s is used.</span>
                <button
                  type="button"
                  className="btn sm danger"
                  aria-disabled={busy || undefined}
                  onClick={goBackToFile}
                >
                  Use the file&apos;s value
                </button>
                <button type="button" className="btn sm" onClick={() => setConfirmFile(false)}>
                  Keep it
                </button>
              </>
            ) : (
              <button type="button" className="btn sm" onClick={() => setConfirmFile(true)}>
                Use the file&apos;s value
              </button>
            )}
          </div>
        ) : null}
        <div className="cp-modal-actions">
          <button type="button" className="btn sm" onClick={onClose}>
            Cancel
          </button>
          {unknown ? null : (
            <button type="submit" className="btn sm primary" aria-disabled={busy || undefined}>
              Add to changes
            </button>
          )}
        </div>
      </form>
    </CaseAlertModal>
  );
}

interface EditorProps {
  spec: LadderKeySpec;
  draft: LadderDraft;
  onChange: (d: LadderDraft) => void;
  /** For a team email list: a field per team. */
  teamRows?: TeamEmailRow[];
  /** Each team field's refusal, by team. */
  errors?: Record<string, string>;
}

/** The editor for one key, by the kind of value it takes. Each caption sits in
 *  a span so its asterisk stays beside it in the label's flex column. */
function Editor({ spec, draft, onChange, teamRows, errors }: EditorProps): JSX.Element {
  const ids = useId();
  const required = ladderValueRequired(spec);
  const text = (type: "text" | "number" | "email" = "text") => (
    <label>
      <span>
        {spec.inputLabel ?? "New value"}
        {required ? <RequiredMark /> : null}
      </span>
      <input
        type={type}
        required={required}
        min={type === "number" ? spec.min : undefined}
        max={type === "number" ? spec.max : undefined}
        value={draft.text}
        placeholder={spec.placeholder}
        onChange={(e) => onChange({ ...draft, text: e.target.value })}
      />
    </label>
  );
  switch (spec.editor) {
    case "bool":
      return (
        <label className="cas-check">
          <input
            type="checkbox"
            checked={draft.checked}
            onChange={(e) => onChange({ ...draft, checked: e.target.checked })}
          />
          On
        </label>
      );
    case "int":
      return text("number");
    case "enum":
      return (
        <label>
          <span>
            New value
            <RequiredMark />
          </span>
          <select required value={draft.text} onChange={(e) => onChange({ ...draft, text: e.target.value })}>
            {(spec.options ?? []).map((o) => (
              <option key={o.value} value={o.value}>
                {o.label}
              </option>
            ))}
          </select>
        </label>
      );
    case "teamEmails":
      if (!teamRows || teamRows.length === 0) {
        return <p className="cas-sub">No ABT teams are listed for this ladder yet.</p>;
      }
      return (
        <table className="cas-team-emails">
          <thead>
            <tr>
              <th scope="col">Team</th>
              <th scope="col">Mailing list</th>
            </tr>
          </thead>
          <tbody>
            {teamRows.map((r) => {
              const id = `${ids}-${r.key}`;
              const err = errors?.[r.key];
              return (
                <tr key={r.key} className={r.stale ? "cas-stale" : undefined}>
                  <th scope="row">
                    <label htmlFor={id}>{r.name}</label>
                    {r.note ? <div className="cas-sub">{r.note}</div> : null}
                  </th>
                  <td>
                    <input
                      id={id}
                      type="email"
                      value={draft.map[r.key] ?? ""}
                      placeholder={spec.placeholder}
                      aria-invalid={err ? true : undefined}
                      aria-describedby={err ? `${id}-err` : undefined}
                      onChange={(e) => onChange({ ...draft, map: { ...draft.map, [r.key]: e.target.value } })}
                    />
                    {err ? (
                      <div id={`${id}-err`} className="cp-err">
                        {err}
                      </div>
                    ) : null}
                  </td>
                </tr>
              );
            })}
          </tbody>
        </table>
      );
    case "head":
      return (
        <>
          <label>
            <span>
              Name
              <RequiredMark />
            </span>
            <input required value={draft.name} onChange={(e) => onChange({ ...draft, name: e.target.value })} />
          </label>
          <label>
            <span>
              Email
              <RequiredMark />
            </span>
            <input
              type="email"
              required
              value={draft.email}
              onChange={(e) => onChange({ ...draft, email: e.target.value })}
            />
          </label>
        </>
      );
    default:
      return text(spec.inputType ?? "text");
  }
}
