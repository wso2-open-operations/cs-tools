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

import { useEffect, useId, useRef, useState, type FormEvent, type JSX } from "react";
import { useTestCaseAlertRoom, type SaveRoomInput } from "../api/useCaseAlertSettings";
import type {
  CaseAlertChatRoom,
  CaseAlertGroupRoute,
  CaseAlertRouteOptions,
  CaseAlertSettings as Settings,
} from "../types";
import { sameTarget, useCaseAlertDrafts, type SettingsFamily } from "../utils/caseAlertDrafts";
import {
  groupRoomOf,
  sameSpaceAs,
  sreGroupDestination,
  sreGroupFallback,
} from "../utils/caseAlertGroupRouting";

const WEBHOOK_PREFIX = "https://chat.googleapis.com/";

/** How many service names a row shows before "+ N more". */
const SERVICES_SHOWN = 3;

function when(iso: string | undefined): string {
  return iso ? new Date(iso).toLocaleString() : "";
}

function capitalized(s: string): string {
  return s.charAt(0).toUpperCase() + s.slice(1);
}

/** A room's name for "same space as": its group's or team's, else its label. */
function roomName(r: CaseAlertChatRoom): string {
  return r.groupName ?? r.label;
}

interface SpaceRowProps {
  family: SettingsFamily;
  /** The group or team the space is for, as the row names it. */
  targetName: string;
  room?: CaseAlertChatRoom;
  /** Every saved room on the tab, for "same space as". */
  rooms: CaseAlertChatRoom[];
  /** The room as it is saved when a URL is added or changed. */
  input: SaveRoomInput;
  /** What happens to its alerts while it has no URL of its own, as a phrase:
   *  "uses the Apollo SRE space", "gets no card: ...". */
  fallback: string;
  canEdit: boolean;
  webhooksEnabled: boolean;
  onError: (msg: string | null) => void;
  onSaved: (what: string) => void;
}

/**
 * One row's Google Chat webhook: the space it posts to (only a hint, never
 * the URL), whether another group or team shares it, its last test, or where
 * its alerts go while none is set; and Add URL / Change URL / Test / Remove.
 * A URL waits in the tab's changes until Submit.
 */
function SpaceCells({
  family,
  targetName,
  room,
  rooms,
  input,
  fallback,
  canEdit,
  webhooksEnabled,
  onError,
  onSaved,
}: SpaceRowProps): JSX.Element {
  const drafts = useCaseAlertDrafts();
  const test = useTestCaseAlertRoom();
  const inputId = useId();
  const [editing, setEditing] = useState(false);
  const [url, setUrl] = useState("");
  const [urlError, setUrlError] = useState<string | null>(null);
  const [confirmRemove, setConfirmRemove] = useState(false);
  const fieldRef = useRef<HTMLInputElement>(null);
  useEffect(() => {
    if (editing) fieldRef.current?.focus();
  }, [editing]);
  const busy = test.isPending || drafts.submitting === family;
  const pending = drafts.changes.find((c) =>
    sameTarget(c, { family, what: "", kind: "room", input }),
  );
  const removing = room
    ? drafts.changes.find((c) => c.kind === "deleteRoom" && c.roomId === room.id)
    : undefined;
  const shared = room ? sameSpaceAs(rooms, room).map(roomName) : [];
  const has = !!room?.hasWebhook && room.enabled;

  const add = (e: FormEvent) => {
    e.preventDefault();
    const value = url.trim();
    if (!value.startsWith(WEBHOOK_PREFIX)) {
      setUrlError(`That isn't a Google Chat webhook URL. It starts with ${WEBHOOK_PREFIX}`);
      return;
    }
    onError(null);
    const what = `${targetName} chat space ${has ? "URL changed" : "added"}`;
    drafts.add({ family, what, kind: "room", input, url: value, roomId: room?.id });
    onSaved(what);
    setEditing(false);
    setUrl("");
    setUrlError(null);
  };

  return (
    <>
      <td>
        {editing ? (
          <form className="cas-url cas-urlform" onSubmit={add} noValidate>
            <label htmlFor={inputId} className="cas-sr">
              Google Chat webhook URL for {targetName}
            </label>
            <input
              id={inputId}
              ref={fieldRef}
              type="url"
              placeholder={`${WEBHOOK_PREFIX}v1/spaces/…`}
              value={url}
              required
              autoComplete="off"
              aria-invalid={urlError ? true : undefined}
              aria-describedby={urlError ? `${inputId}-err` : undefined}
              onChange={(e) => {
                setUrl(e.target.value);
                setUrlError(null);
              }}
            />
            {urlError ? (
              <div id={`${inputId}-err`} className="cas-sub cas-warn" role="alert">
                {urlError}
              </div>
            ) : null}
            <div className="cas-acts">
              <button type="submit" className="btn sm primary" disabled={!url.trim()}>
                Add to changes
              </button>
              <button
                type="button"
                className="btn sm"
                onClick={() => {
                  setEditing(false);
                  setUrl("");
                  setUrlError(null);
                }}
              >
                Cancel
              </button>
            </div>
          </form>
        ) : has && room ? (
          <>
            <code className="cas-hint">{room.webhookHint}</code>
            {shared.length > 0 ? (
              <div className="cas-sub">same space as {shared.join(", ")}</div>
            ) : null}
            {room.lastTestStatus ? (
              <div className="cas-sub">
                Last test: <span className={`cas-test ${room.lastTestStatus}`}>{room.lastTestStatus}</span>
                {room.lastTestAt ? ` · ${when(room.lastTestAt)}` : ""}
              </div>
            ) : null}
          </>
        ) : (
          <span className="cas-sub">Not set · {fallback}</span>
        )}
        {pending ? (
          <div className="cas-sub">
            <span className="cas-chip">New URL waits for Submit</span>
            <button type="button" className="cas-link" onClick={() => drafts.drop(pending.id)}>
              Undo
            </button>
          </div>
        ) : null}
        {removing ? (
          <div className="cas-sub">
            <span className="cas-chip">Removed when you Submit</span>
            <button type="button" className="cas-link" onClick={() => drafts.drop(removing.id)}>
              Undo
            </button>
          </div>
        ) : null}
      </td>
      {canEdit ? (
        <td>
          {editing ? null : confirmRemove ? (
            <div className="cas-acts">
              <span className="cas-sub">Remove? It then {fallback}.</span>
              <button
                type="button"
                className="btn sm danger"
                disabled={busy}
                onClick={() => {
                  if (!room) return;
                  onError(null);
                  setConfirmRemove(false);
                  const what = `${targetName} chat space removed`;
                  drafts.add({ family, what, kind: "deleteRoom", roomId: room.id });
                  onSaved(what);
                }}
              >
                Remove
              </button>
              <button type="button" className="btn sm" onClick={() => setConfirmRemove(false)}>
                Cancel
              </button>
            </div>
          ) : (
            <div className="cas-acts">
              <button
                type="button"
                className="btn sm"
                disabled={busy || !webhooksEnabled}
                aria-label={`${has ? "Change the chat space URL of" : "Add a chat space URL for"} ${targetName}`}
                onClick={() => setEditing(true)}
              >
                {has ? "Change URL" : "Add URL"}
              </button>
              {has && room ? (
                <button
                  type="button"
                  className="btn sm"
                  disabled={busy || !webhooksEnabled}
                  aria-label={`Send a test card to ${targetName}'s chat space`}
                  onClick={() => {
                    onError(null);
                    test.mutate(room.id, {
                      onError: (e: Error) => onError(e.message || "The test card was not sent."),
                    });
                  }}
                >
                  {test.isPending ? "Sending…" : "Test"}
                </button>
              ) : null}
              {room && !removing ? (
                <button
                  type="button"
                  className="btn sm"
                  disabled={busy}
                  aria-label={`Remove ${targetName}'s chat space`}
                  onClick={() => setConfirmRemove(true)}
                >
                  Remove
                </button>
              ) : null}
            </div>
          )}
        </td>
      ) : null}
    </>
  );
}

/** A row's services: the first few, and how many more. */
function Services({ names }: { names?: string[] }): JSX.Element {
  if (!names || names.length === 0) return <span className="cas-sub">—</span>;
  const shown = names.slice(0, SERVICES_SHOWN);
  const more = names.length - shown.length;
  return (
    <span title={names.join(", ")}>
      {shown.join(", ")}
      {more > 0 ? <span className="cas-sub"> + {more} more</span> : null}
    </span>
  );
}

interface GroupChatSpacesProps {
  settings: Settings;
  options?: CaseAlertRouteOptions;
  /** Every saved room the tab shows (SRE and SME), for "same space as". */
  rooms: CaseAlertChatRoom[];
  canEdit: boolean;
  onError: (msg: string | null) => void;
  onSaved: (what: string) => void;
}

/**
 * Before escalation: every SRE assignment group (the support group of a
 * service), the SRE sub team it is paged as (for reference: changing it is a
 * redirect, under Advanced), its services, and the Google Chat space its
 * incident cards and the paging replies under them go to. Only the space is
 * set here; who is called does not change.
 */
export function GroupChatSpaces({
  settings: s,
  options,
  rooms,
  canEdit,
  onError,
  onSaved,
}: GroupChatSpacesProps): JSX.Element {
  const lower = (x: string) => x.trim().toLowerCase();
  const routes: CaseAlertGroupRoute[] = s.routes.filter((r) => r.ladder === "SRE");
  const destination = sreGroupDestination(s, options);
  const groups = [...(options?.assignmentGroups ?? [])].sort((a, b) => a.name.localeCompare(b.name));
  const other = destination("");

  return (
    <div className="cas-block" role="group" aria-label="Before escalation">
      <h4>Before escalation</h4>
      <p className="cas-sub">Each assignment group, the SRE sub team it maps to, and its chat space.</p>
      {groups.length === 0 ? (
        <p className="cas-sub cas-empty">No assignment group is the support group of a service yet.</p>
      ) : (
        <table className="ra-tbl cas-groups cas-spaces" aria-label="Each assignment group's chat space">
          <thead>
            <tr>
              <th>Assignment group</th>
              <th>SRE sub team</th>
              <th>Its services</th>
              <th>Google Chat webhook</th>
              {canEdit ? <th aria-label="Actions" /> : null}
            </tr>
          </thead>
          <tbody>
            {groups.map((g) => {
              const route = routes.find(
                (r) => (r.groupId && r.groupId === g.id) || lower(r.groupName) === lower(g.name),
              );
              const dest = destination(g.name, route);
              const room = groupRoomOf(rooms, g.id);
              return (
                <tr key={g.id} className={dest.unmapped ? "cas-unmapped" : undefined}>
                  <td>
                    <b>{g.name}</b>
                  </td>
                  <td>{dest.name}</td>
                  <td>
                    <Services names={g.services} />
                  </td>
                  <SpaceCells
                    family="SRE"
                    targetName={g.name}
                    room={room}
                    rooms={rooms}
                    input={{ ladder: "SRE", groupId: g.id, label: room?.label ?? g.name }}
                    fallback={sreGroupFallback(s, rooms, dest)}
                    canEdit={canEdit}
                    webhooksEnabled={s.webhooksEnabled}
                    onError={onError}
                    onSaved={onSaved}
                  />
                </tr>
              );
            })}
            <tr className="cas-other">
              <td>
                <i>Any other group</i>
              </td>
              <td>Not mapped</td>
              <td>
                <span className="cas-sub">—</span>
              </td>
              <td>
                <span className="cas-sub">{capitalized(sreGroupFallback(s, rooms, other))}</span>
              </td>
              {canEdit ? <td /> : null}
            </tr>
          </tbody>
        </table>
      )}
      <p className="cas-sub">
        Every support group from ServiceNow, with its services. The SRE sub team is shown for
        reference; only the chat space is set here.
      </p>
    </div>
  );
}

interface SpecialistChatSpacesProps {
  settings: Settings;
  /** The Team Schedule's SME teams. */
  teams: readonly { key: string; name: string }[];
  rooms: CaseAlertChatRoom[];
  canEdit: boolean;
  onError: (msg: string | null) => void;
  onSaved: (what: string) => void;
}

/**
 * After escalation: each SME sub team and the Google Chat space its alerts go
 * to once an incident is escalated to it. A team with no space of its own
 * uses its space in the alert service's file, else the shared SME space.
 */
export function SpecialistChatSpaces({
  settings: s,
  teams,
  rooms,
  canEdit,
  onError,
  onSaved,
}: SpecialistChatSpacesProps): JSX.Element {
  const running = s.running?.channels?.SME?.teams ?? [];
  const sorted = [...teams].sort((a, b) => a.name.localeCompare(b.name));
  return (
    <div className="cas-block" role="group" aria-label="After escalation">
      <h4>After escalation</h4>
      <p className="cas-sub">Each SME sub team and its chat space.</p>
      {sorted.length === 0 ? (
        <p className="cas-sub cas-empty">The Team Schedule has no SME team yet.</p>
      ) : (
        <table className="ra-tbl cas-spaces" aria-label="Each SME sub team's chat space">
          <thead>
            <tr>
              <th>SME sub team (who is called)</th>
              <th>Google Chat webhook</th>
              {canEdit ? <th aria-label="Actions" /> : null}
            </tr>
          </thead>
          <tbody>
            {sorted.map((t) => {
              const room = rooms.find((r) => r.ladder === "SME" && r.teamKey === t.key);
              const fileRoom = running.some((x) => x.team === t.key && x.room);
              return (
                <tr key={t.key}>
                  <td>
                    <b>{t.name}</b>
                    {t.name.toLowerCase() !== t.key.toLowerCase() ? (
                      <div className="cas-sub">{t.key}</div>
                    ) : null}
                  </td>
                  <SpaceCells
                    family="SRE"
                    targetName={t.name}
                    room={room}
                    rooms={rooms}
                    input={{ ladder: "SME", teamKey: t.key, label: room?.label ?? `${t.name} SME` }}
                    fallback={
                      fileRoom
                        ? "uses its space from the alert service's file"
                        : "uses the shared SME space from the alert service's file"
                    }
                    canEdit={canEdit}
                    webhooksEnabled={s.webhooksEnabled}
                    onError={onError}
                    onSaved={onSaved}
                  />
                </tr>
              );
            })}
          </tbody>
        </table>
      )}
      <p className="cas-sub">
        A sub team is called only when an incident is escalated to it. WSO2 IaaS and default-group
        incidents are not escalated.
      </p>
    </div>
  );
}
