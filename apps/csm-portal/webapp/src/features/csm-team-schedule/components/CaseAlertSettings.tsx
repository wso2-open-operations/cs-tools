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

import {
  useContext,
  useEffect,
  useId,
  useRef,
  useState,
  type FormEvent,
  type JSX,
  type KeyboardEvent,
  type RefObject,
} from "react";
import {
  useCaseAlertSettings,
  useTestCaseAlertRoom,
} from "../api/useCaseAlertSettings";
import type {
  CaseAlertChannel,
  CaseAlertChatRoom,
  CaseAlertGroupRoute,
  CaseAlertLadder,
  CaseAlertRoomLadder,
  CaseAlertRouteOptions,
  CaseAlertSettings as Settings,
  CaseAlertTeamOption,
  PagingChainCode,
  ScheduleTeam,
} from "../types";
import {
  FAMILY_LADDERS,
  familyOfChain,
  familyOfLadder,
} from "../utils/caseAlertChainSettings";
import {
  DraftsContext,
  useCaseAlertDrafts,
  type DraftChange,
  type SettingsFamily,
} from "../utils/caseAlertDrafts";
import {
  channelWords,
  teamChannelsUnder,
  valuesToCopy,
} from "../utils/caseAlertLadderKeys";
import CaseAlertDraftsProvider from "./CaseAlertDraftsProvider";
import CaseAlertLadderSettings from "./CaseAlertLadderSettings";
import CaseAlertModal from "./CaseAlertModal";
import CaseAlertSection from "./CaseAlertSection";

/** What a new room starts as, as the alert service's file has it today: its
 *  label, and whether it also gets the incidents with no SRE team (the SaaS
 *  teams do), so adding one never quietly changes where those go. Keyed by
 *  ladder, then team key ("" for the shared space). */
const ROOM_DEFAULTS: Record<
  CaseAlertRoomLadder,
  Record<string, { label: string; teamless?: boolean }>
> = {
  CRE: { "": { label: "CRE Case Alerts" } },
  SRE: {
    "": { label: "SRE Case Alerts" },
    apollo: { label: "Apollo SRE", teamless: true },
    artemis: { label: "Artemis SRE", teamless: true },
    iaas: { label: "IaaS SRE" },
  },
  // An SME team's room is "<team> SME", as the file's audiences are.
  SME: {},
};


const WEBHOOK_PREFIX = "https://chat.googleapis.com/";

function when(iso: string | undefined): string {
  return iso ? new Date(iso).toLocaleString() : "";
}

/** What the alert service is doing with these settings, in one line. It is
 *  what the service last reported at start-up: switched back off, it no
 *  longer asks or reports, so the time is part of the answer. */
function statusLine(s: Settings): { tone: "info" | "warn"; text: string } {
  const r = s.revision;
  const asOf = r.appliedAt
    ? ` (as the alert service reported at ${when(r.appliedAt)})`
    : "";
  switch (r.appliedMode) {
    case "db":
      return r.applied === r.saved
        ? { tone: "info", text: `In use by the alert service${asOf}.` }
        : {
            tone: "warn",
            text: `Saved. Changes apply the next time the alert service restarts${asOf}.`,
          };
    case "shadow":
      return {
        tone: "warn",
        text: `Saved, not in use yet. The alert service is comparing these with its own settings, but still uses its own${asOf}.`,
      };
    case "off":
      // Reported "off": the saved settings were read but refused by the
      // file's own checks, so the alert service runs on its file.
      return {
        tone: "warn",
        text: `Saved, not in use: at its last start the alert service could not use these and runs on its own file${asOf}.`,
      };
    default:
      return {
        tone: "warn",
        text: "The alert service uses the settings saved here from its next restart.",
      };
  }
}

interface Props {
  /** The family on screen: CRE, or SRE (SaaS, IaaS and SME together). */
  family?: SettingsFamily;
  /** A chain on screen instead: its family is shown. */
  chain?: PagingChainCode;
  /** The Team Schedule's teams (the catalogue): where an SME room's team is
   *  picked from, since the route options hold CRE and SRE teams only. */
  teams?: readonly Pick<ScheduleTeam, "key" | "name" | "family">[];
  /** Teams people are only moved onto (Migration, SaaS Night cover): they
   *  are never alerted as themselves, so they get no chat space or redirect. */
  moveTargetKeys?: readonly string[];
}

/**
 * The Case Alerts settings of one family, under the tab's Settings: a header
 * saying what the alert service does with them, each team's Google Chat space,
 * which team other CSM Portal teams' alerts go to, and the rest of each
 * ladder's paging file (CaseAlertLadderSettings). Every change waits in the
 * tab's list until its Submit, which saves them together. Shown only to those
 * the server lets change the family's settings (canManage), and each row only
 * changeable by those who may change its ladder.
 *
 * A webhook URL is pasted once and never shown again: the page only ever
 * holds a hint. A room or route not saved here leaves the alert service on
 * its own setting for it.
 */
export default function CaseAlertSettings(props: Props): JSX.Element | null {
  // On its own (not under the Alert Settings tab), it keeps its own changes.
  const ctx = useContext(DraftsContext);
  return ctx ? (
    <SettingsPanel {...props} />
  ) : (
    <CaseAlertDraftsProvider>
      <SettingsPanel {...props} />
    </CaseAlertDraftsProvider>
  );
}

function SettingsPanel({
  family: familyProp,
  chain,
  teams = [],
  moveTargetKeys = [],
}: Props): JSX.Element | null {
  const settings = useCaseAlertSettings();
  const drafts = useCaseAlertDrafts();
  const [error, setError] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);
  const family = familyProp ?? (chain ? familyOfChain(chain) : "SRE");
  const s = settings.data;
  if (!s || !s.canManage.view) return null;

  const ladders = FAMILY_LADDERS[family];
  // A family's settings are seen and changed by the same people; the server
  // sends nobody another family's.
  const editable = ladders.filter((l) => s.canManage.roomLadders.includes(l));
  if (editable.length === 0) return null;
  const hidden = new Set(moveTargetKeys.map((k) => k.toLowerCase()));
  const alerted = (t: CaseAlertTeamOption) => !hidden.has(t.key.toLowerCase());
  const routeOptions: CaseAlertRouteOptions | undefined = s.routeOptions && {
    ...s.routeOptions,
    teams: s.routeOptions.teams.filter(alerted),
  };
  const roomOptions: CaseAlertRouteOptions = {
    groups: [],
    rotas: [],
    teams: [
      ...(routeOptions?.teams ?? []).filter((t) => t.family !== "SME" && ladders.includes(t.family)),
      ...(ladders.includes("SME")
        ? teams
            .filter((t) => t.family === "SME")
            .map((t) => ({ key: t.key, name: t.name, family: "SME" as const }))
            .filter(alerted)
        : []),
    ],
  };
  const rooms = s.rooms.filter((r) => ladders.includes(r.ladder));
  const status = statusLine(s);
  // The ladder's own channel is the file's; a team can only do less.
  const ladderChannel = s.running?.fileOnly?.[family === "CRE" ? "CRE" : "SRE"]?.channel;
  const pending = drafts.changes.filter((c) => c.family === family);
  /** A change was made: say it waits for Submit, in place of the last line. */
  const onSaved = (what: string) => {
    setError(null);
    setNotice(`Added to this tab's changes: ${what}. Submit to save them.`);
  };
  const submit = async () => {
    setError(null);
    setNotice(null);
    const r = await drafts.submit(family);
    if (r.saved > 0) {
      setNotice(`✓ Saved ${r.saved} ${r.saved === 1 ? "change" : "changes"}. ${whenUsed(s)}`);
    }
    if (r.error) setError(`${r.error} Nothing after it was saved; it is still in the list.`);
  };

  return (
    <section className="cas" aria-labelledby={`cas-h-${family}`}>
      <div className="cas-head">
        <h3 id={`cas-h-${family}`}>{family} settings</h3>
        {family === "SRE" ? <span className="cas-sub">SaaS, IaaS and SME</span> : null}
        {s.environment ? (
          <span className="cas-env">Environment: {s.environment}</span>
        ) : null}
      </div>
      <p className={`cas-status ${status.tone}`}>{status.text}</p>
      <div className="cas-live" aria-live="polite">
        {notice ? (
          <p className="cas-status ok cas-done">
            <span>{notice}</span>
            <button
              type="button"
              className="cas-x"
              aria-label="Dismiss"
              onClick={() => setNotice(null)}
            >
              ×
            </button>
          </p>
        ) : null}
      </div>
      {error ? (
        <div className="ra-err" role="alert">
          {error}
        </div>
      ) : null}
      {editable.map((l) => (
        <SetupStep key={l} settings={s} ladder={l} onError={setError} onSaved={onSaved} />
      ))}

      <TeamChatRouting
        ladders={ladders}
        editable={editable}
        rooms={rooms}
        options={roomOptions}
        ladderChannel={ladderChannel}
        roomsOnly
        webhooksEnabled={s.webhooksEnabled}
        appliedNote={whenUsed(s)}
        onError={setError}
        onSaved={onSaved}
      />

      {editable
        .filter((l): l is CaseAlertLadder => l !== "SME")
        .map((l) => (
          <Redirects
            key={l}
            ladder={l}
            settings={{ ...s, routeOptions }}
            rooms={rooms.filter((r) => r.ladder === l)}
            ladderChannel={ladderChannel}
            onError={setError}
            onSaved={onSaved}
          />
        ))}
      {editable.map((l) => (
        <CaseAlertLadderSettings key={l} ladder={l} settings={s} onSaved={onSaved} />
      ))}

      <SubmitBar
        family={family}
        changes={pending}
        submitting={drafts.submitting === family}
        onSubmit={() => void submit()}
        onDiscard={() => {
          drafts.discard(family);
          setNotice(null);
        }}
        onDrop={drafts.drop}
      />
    </section>
  );
}

interface SubmitBarProps {
  family: SettingsFamily;
  changes: DraftChange[];
  submitting: boolean;
  onSubmit: () => void;
  onDiscard: () => void;
  onDrop: (id: number) => void;
}

/** The family's changes not saved yet, and its Submit: nothing above is saved
 *  until it is pressed. */
function SubmitBar({
  family,
  changes,
  submitting,
  onSubmit,
  onDiscard,
  onDrop,
}: SubmitBarProps): JSX.Element {
  const n = changes.length;
  return (
    <div className="cas-submit" role="region" aria-label={`${family} changes to submit`}>
      {n === 0 ? (
        <p className="cas-sub">No changes to submit.</p>
      ) : (
        <>
          <p className="cas-sub">
            {n} {n === 1 ? "change" : "changes"} not saved yet:
          </p>
          <ul className="cas-pending">
            {changes.map((c) => (
              <li key={c.id}>
                <span>{c.what}</span>
                <button
                  type="button"
                  className="cas-x"
                  aria-label={`Undo: ${c.what}`}
                  disabled={submitting}
                  onClick={() => onDrop(c.id)}
                >
                  ×
                </button>
              </li>
            ))}
          </ul>
        </>
      )}
      <div className="cas-submit-acts">
        {n > 0 ? (
          <button type="button" className="btn sm" disabled={submitting} onClick={onDiscard}>
            Discard
          </button>
        ) : null}
        <button
          type="button"
          className="btn sm primary"
          disabled={n === 0 || submitting}
          onClick={onSubmit}
        >
          {submitting ? "Submitting…" : `Submit ${family} changes`}
        </button>
      </div>
    </div>
  );
}

interface RedirectsProps {
  ladder: CaseAlertLadder;
  settings: Settings;
  rooms: CaseAlertChatRoom[];
  ladderChannel?: string;
  onError: (msg: string | null) => void;
  onSaved: (what: string) => void;
}

/** A ladder's redirects: which team a CSM Portal team's cases (CRE) or
 *  incidents (SRE) go to. */
function Redirects({
  ladder,
  settings: s,
  rooms,
  ladderChannel,
  onError,
  onSaved,
}: RedirectsProps): JSX.Element {
  const canRoutes = s.canManage.routeLadders.includes(ladder);
  const routes = s.routes.filter((r) => r.ladder === ladder);
  return (
    <>
      <CaseAlertSection
        id={`${ladder}.redirects`}
        label={ROUTE_WORDS[ladder].title}
        summary={`${ROUTE_WORDS[ladder].title} · ${
          routes.length === 0
            ? "none"
            : `${routes.length} redirect${routes.length === 1 ? "" : "s"}`
        }`}
      >
        <p className="cas-sub">{ROUTE_WORDS[ladder].help}</p>
        <RouteTable
          ladder={ladder}
          routes={routes}
          options={s.routeOptions}
          rooms={rooms}
          ladderChannel={ladderChannel}
          canEdit={canRoutes}
          appliedNote={whenUsed(s)}
          onError={onError}
          onSaved={onSaved}
        />
      </CaseAlertSection>
    </>
  );
}

/** whenUsed's usual answer, for a dialog not told the mode. */
const DEFAULT_APPLIED_NOTE = "The alert service uses it after its next restart.";

/** When the alert service starts using a saved change. */
function whenUsed(s: Settings): string {
  // In step with statusLine: what the alert service does with a saved change.
  switch (s.revision.appliedMode) {
    case "shadow":
      return "The alert service only compares it with its file (shadow mode); it uses it once PAGING_CONFIG_SOURCE is db or unset.";
    case "off":
      return "The alert service could not use the saved settings at its last start; it uses them after a restart that accepts them.";
    default:
      return DEFAULT_APPLIED_NOTE;
  }
}

/** A team's name for people: the shared space, else the Team Schedule's
 *  name for the team, else the room's own label. */
function roomTeamName(
  ladder: CaseAlertRoomLadder,
  teamKey: string | undefined,
  options: CaseAlertRouteOptions | undefined,
  fallback: string,
): string {
  if (!teamKey) return `All ${ladder} teams (shared space)`;
  return options?.teams.find((t) => t.key === teamKey)?.name ?? fallback;
}

/** What a team's chat routing is called in the confirmation line. */
function routingWhat(teamName: string): string {
  return `${teamName.replace(/ \(shared space\)$/, "")} chat routing`;
}

/** A team (or a ladder's shared space) the Add dialog offers: every one of
 *  the reader's ladders, whether or not it has a space yet -- picking one that
 *  has opens it to change. */
interface RoomChoice {
  value: string;
  label: string;
  ladder: CaseAlertRoomLadder;
  teamKey?: string;
  room?: CaseAlertChatRoom;
}

interface TeamChatRoutingProps {
  /** The ladders whose rooms this tab shows (SRE's tab: SRE and SME). */
  ladders: CaseAlertRoomLadder[];
  /** Of those, the ones the reader may change. */
  editable: CaseAlertRoomLadder[];
  /** The rooms saved here for those ladders. */
  rooms: CaseAlertChatRoom[];
  /** The teams of those ladders, as the Team Schedule names them. */
  options?: CaseAlertRouteOptions;
  ladderChannel?: string;
  /** Only the chat space is set here: how a team is alerted stays the file's. */
  roomsOnly: boolean;
  webhooksEnabled: boolean;
  /** When the alert service starts using a saved change (whenUsed). */
  appliedNote: string;
  onError: (msg: string | null) => void;
  onSaved: (what: string) => void;
}

/**
 * Team chat routing: which Google Chat space each team's alerts go to. Only
 * routings saved here are listed; every other team uses the alert service's
 * file. Each row can be changed by those who may change its ladder (SRE rows
 * by the SRE rota admin, SME rows by the SME rota admin). Adding and changing
 * go through a dialog; the change waits for the tab's Submit.
 */
function TeamChatRouting({
  ladders,
  editable,
  rooms,
  options,
  ladderChannel,
  roomsOnly,
  webhooksEnabled,
  appliedNote,
  onError,
  onSaved,
}: TeamChatRoutingProps): JSX.Element {
  const [adding, setAdding] = useState(false);
  const addRef = useRef<HTMLButtonElement>(null);
  const ladderWords = channelWords(ladderChannel);
  const canAdd = editable.length > 0;
  const name = (r: CaseAlertChatRoom) =>
    roomTeamName(r.ladder, r.teamKey, options, r.label);
  // Each ladder's shared space first, then its teams by name.
  const order = (r: CaseAlertChatRoom) =>
    `${ladders.indexOf(r.ladder)}${r.teamKey ? "1" : "0"}${name(r).toLowerCase()}`;
  const sorted = [...rooms].sort((a, b) => order(a).localeCompare(order(b)));
  // Every team of the ladders the reader may change, with or without a space.
  const roomOf = (ladder: CaseAlertRoomLadder, teamKey?: string) =>
    rooms.find((r) => r.ladder === ladder && (r.teamKey ?? "") === (teamKey ?? ""));
  const choices: RoomChoice[] = editable.flatMap((ladder) => {
    const shared: RoomChoice[] =
      ladder === "SME"
        ? []
        : [
            {
              value: `${ladder}|`,
              label: `All ${ladder} teams (shared space)`,
              ladder,
              room: roomOf(ladder),
            },
          ];
    const teams = (options?.teams ?? [])
      .filter((t) => t.family === ladder)
      .sort((a, b) => a.name.localeCompare(b.name))
      .map((t) => ({
        value: `${ladder}|${t.key}`,
        label: t.name,
        ladder,
        teamKey: t.key,
        room: roomOf(ladder, t.key),
      }));
    return [...shared, ...teams];
  });
  const addBlocked = !webhooksEnabled
    ? "Webhook URLs can't be saved on this server yet"
    : undefined;

  return (
    <>
      <div className="cas-h4row">
        <h4>Team chat routing</h4>
        {canAdd ? (
          <button
            ref={addRef}
            type="button"
            className="btn sm primary"
            disabled={addBlocked !== undefined}
            title={addBlocked}
            onClick={() => setAdding(true)}
          >
            + Add team chat routing
          </button>
        ) : null}
      </div>
      <p className="cas-sub">
        {roomsOnly
          ? "Which Google Chat space each team's alerts go to."
          : "Which Google Chat space each team's alerts go to, and whether the team gets chat cards, calls or both."}
        {!roomsOnly && ladderWords
          ? ` This ladder sends alerts by ${ladderWords.toLowerCase()} (set in the alert service's file). A team can do less, never more.`
          : ""}
      </p>
      {!webhooksEnabled ? (
        <p className="cas-sub cas-warn">
          Webhook URLs can&apos;t be saved on this server.
        </p>
      ) : null}
      {sorted.length === 0 ? (
        <p className="cas-sub cas-empty">
          No team chat routing saved here yet. Alerts go to the chat spaces in
          the alert service&apos;s file.
        </p>
      ) : (
        <table className="ra-tbl cas-rooms" aria-label="Team chat routing">
          <thead>
            <tr>
              <th>Team</th>
              <th>Google Chat space</th>
              {roomsOnly ? null : <th>Alerts by</th>}
              {canAdd ? <th aria-label="Actions" /> : null}
            </tr>
          </thead>
          <tbody>
            {sorted.map((room) => (
              <ChatRow
                key={room.id}
                ladder={room.ladder}
                room={room}
                teamName={name(room)}
                ladderChannel={ladderChannel}
                canEdit={editable.includes(room.ladder)}
                showActions={canAdd}
                roomsOnly={roomsOnly}
                webhooksEnabled={webhooksEnabled}
                appliedNote={appliedNote}
                onError={onError}
                onSaved={onSaved}
              />
            ))}
          </tbody>
        </table>
      )}
      {adding ? (
        <RoomDialog
          ladder={choices[0]?.ladder ?? editable[0]}
          choices={choices}
          options={options}
          ladderChannel={ladderChannel}
          roomsOnly={roomsOnly}
          webhooksEnabled={webhooksEnabled}
          appliedNote={appliedNote}
          returnFocus={addRef}
          onClose={() => setAdding(false)}
          onSaved={onSaved}
        />
      ) : null}
    </>
  );
}

interface ChatRowProps {
  ladder: CaseAlertRoomLadder;
  room: CaseAlertChatRoom;
  teamName: string;
  ladderChannel?: string;
  canEdit: boolean;
  /** Whether the table has an actions column (somebody may change a row). */
  showActions?: boolean;
  roomsOnly: boolean;
  webhooksEnabled: boolean;
  appliedNote?: string;
  onError: (msg: string | null) => void;
  onSaved: (what: string) => void;
}

/** One saved chat routing, read-only: changes go through Edit and its Save. */
function ChatRow({
  ladder,
  room,
  teamName,
  ladderChannel,
  canEdit,
  showActions = canEdit,
  roomsOnly,
  webhooksEnabled,
  appliedNote,
  onError,
  onSaved,
}: ChatRowProps): JSX.Element {
  const drafts = useCaseAlertDrafts();
  const family = familyOfLadder(ladder);
  const test = useTestCaseAlertRoom();
  const [editing, setEditing] = useState(false);
  const editRef = useRef<HTMLButtonElement>(null);
  const [confirm, setConfirm] = useState<"off" | "remove" | null>(null);
  const confirmRef = useRef<HTMLButtonElement>(null);
  // The menu that asked is gone: the question takes focus.
  useEffect(() => {
    if (confirm) confirmRef.current?.focus();
  }, [confirm]);
  const busy = test.isPending || drafts.submitting === family;
  const fail = (e: Error) => onError(e.message || "That change did not save.");
  const what = routingWhat(teamName);
  const ladderWords = channelWords(ladderChannel);
  const alertsBy = room.teamKey
    ? room.channel
      ? channelWords(room.channel)
      : ladderWords
        ? `Same as the ladder (${ladderWords})`
        : "Same as the ladder"
    : (ladderWords ?? "—");

  /** Turns the room on or off, sending every field so nothing else moves. */
  const setEnabled = (enabled: boolean) => {
    onError(null);
    setConfirm(null);
    const change = `${what} turned ${enabled ? "on" : "off"}`;
    drafts.add({
      family,
      what: change,
      kind: "room",
      roomId: room.id,
      input: {
        ladder,
        teamKey: room.teamKey,
        label: room.label,
        enabled,
        teamless: room.teamless,
        ...(room.teamKey ? { channel: room.channel ?? "" } : {}),
      },
    });
    onSaved(change);
  };

  return (
    <tr className={room.enabled ? undefined : "cas-off"}>
      <td>
        <b>{teamName}</b>
        <div className="cas-sub">
          {room.label}
          {room.teamless && room.teamKey
            ? " · also gets incidents with no SRE team"
            : ""}
          {!room.enabled ? <span className="cas-chip">Off</span> : null}
        </div>
      </td>
      <td>
        {room.hasWebhook ? (
          <>
            <code className="cas-hint">{room.webhookHint}</code>
            {room.lastTestStatus ? (
              <div className="cas-sub">
                Last test:{" "}
                <span className={`cas-test ${room.lastTestStatus}`}>
                  {room.lastTestStatus}
                </span>
                {room.lastTestAt ? ` · ${when(room.lastTestAt)}` : ""}
              </div>
            ) : null}
          </>
        ) : (
          <span className="cas-sub">
            No URL saved (uses the file&apos;s space)
          </span>
        )}
      </td>
      {roomsOnly ? null : (
        <td>
          <span>{alertsBy}</span>
          {!room.teamKey && ladderWords ? (
            <span className="cas-chip">set in the file</span>
          ) : null}
        </td>
      )}
      {canEdit ? (
        <td>
          <div className="cas-acts">
            {confirm ? (
              <>
                <span className="cas-sub">
                  {confirm === "off"
                    ? "Turn off? Its alerts use the file's space."
                    : "Remove? Its alerts use the file's space."}
                </span>
                <button
                  type="button"
                  className="btn sm danger"
                  disabled={busy}
                  ref={confirmRef}
                  onClick={() => {
                    if (confirm === "off") {
                      setEnabled(false);
                      return;
                    }
                    onError(null);
                    setConfirm(null);
                    drafts.add({ family, what: `${what} removed`, kind: "deleteRoom", roomId: room.id });
                    onSaved(`${what} removed`);
                  }}
                >
                  {confirm === "off" ? "Turn off" : "Remove"}
                </button>
                <button
                  type="button"
                  className="btn sm"
                  onClick={() => setConfirm(null)}
                >
                  Cancel
                </button>
              </>
            ) : (
              <>
                <button
                  ref={editRef}
                  type="button"
                  className="btn sm"
                  disabled={busy}
                  onClick={() => setEditing(true)}
                >
                  Edit
                </button>
                {room.hasWebhook ? (
                  <button
                    type="button"
                    className="btn sm"
                    disabled={busy || !webhooksEnabled}
                    onClick={() => {
                      onError(null);
                      test.mutate(room.id, { onError: fail });
                    }}
                  >
                    {test.isPending ? "Sending…" : "Test"}
                  </button>
                ) : null}
                <MoreMenu
                  disabled={busy}
                  items={[
                    room.enabled
                      ? {
                          label: "Turn off",
                          onSelect: () => setConfirm("off"),
                        }
                      : {
                          label: "Turn on",
                          onSelect: () => setEnabled(true),
                        },
                    { label: "Remove", onSelect: () => setConfirm("remove") },
                  ]}
                />
              </>
            )}
          </div>
          {editing ? (
            <RoomDialog
              ladder={ladder}
              room={room}
              teamName={teamName}
              ladderChannel={ladderChannel}
              roomsOnly={roomsOnly}
              webhooksEnabled={webhooksEnabled}
              appliedNote={appliedNote}
              returnFocus={editRef}
              onClose={() => setEditing(false)}
              onSaved={onSaved}
            />
          ) : null}
        </td>
      ) : showActions ? (
        <td />
      ) : null}
    </tr>
  );
}

interface RoomDialogProps {
  ladder: CaseAlertRoomLadder;
  /** Editing this room; absent when adding one. */
  room?: CaseAlertChatRoom;
  /** The room's team, as the row names it (edit only). */
  teamName?: string;
  /** The teams the reader may pick from (add only). */
  choices?: RoomChoice[];
  options?: CaseAlertRouteOptions;
  ladderChannel?: string;
  /** Only the space is set: a team's channel is kept as it is. */
  roomsOnly: boolean;
  webhooksEnabled: boolean;
  /** When the alert service starts using a saved change (whenUsed). */
  appliedNote?: string;
  returnFocus: RefObject<HTMLButtonElement | null>;
  onClose: () => void;
  onSaved: (what: string) => void;
}

/**
 * Add a team chat routing, or change one: the team, its space's webhook URL,
 * how it is alerted and whether it also gets incidents with no SRE team.
 * Save sends the room with every field, then the URL when one was typed.
 * Errors stay in the dialog.
 */
function RoomDialog({
  ladder: ladderProp,
  room,
  teamName,
  choices = [],
  options,
  ladderChannel,
  roomsOnly,
  webhooksEnabled,
  appliedNote = DEFAULT_APPLIED_NOTE,
  returnFocus,
  onClose,
  onSaved,
}: RoomDialogProps): JSX.Element {
  const drafts = useCaseAlertDrafts();
  const ids = useId();
  const titleId = `${ids}-title`;
  const [pick, setPick] = useState(room ? "" : (choices[0]?.value ?? ""));
  const choice = room ? undefined : choices.find((c) => c.value === pick);
  // Picking a team that has a space already changes that space.
  const existing = room ?? choice?.room;
  const ladder: CaseAlertRoomLadder = room?.ladder ?? choice?.ladder ?? ladderProp;
  const teamKey = room ? room.teamKey : choice?.teamKey;
  const defaults = ROOM_DEFAULTS[ladder];
  const [url, setUrl] = useState("");
  const [channel, setChannel] = useState<CaseAlertChannel | "">(
    existing?.channel ?? "",
  );
  const [teamless, setTeamless] = useState(
    existing ? existing.teamless : Boolean(defaults[teamKey ?? ""]?.teamless),
  );
  const [error, setError] = useState<string | null>(null);
  const ladderWords = channelWords(ladderChannel);
  const allowed = teamChannelsUnder(ladderChannel);
  const name =
    teamName ??
    roomTeamName(ladder, teamKey, options, choice?.label ?? teamKey ?? "");

  const pickTeam = (value: string) => {
    setPick(value);
    const c = choices.find((x) => x.value === value);
    // A team with a space keeps its own; a new one starts as the file has it.
    setTeamless(
      c?.room ? c.room.teamless : Boolean(c && ROOM_DEFAULTS[c.ladder][c.teamKey ?? ""]?.teamless),
    );
    setChannel(c?.room?.channel ?? "");
  };

  const submit = (e: FormEvent) => {
    e.preventDefault();
    setError(null);
    const value = url.trim();
    // A new space needs its URL; one saved before keeps whatever it has.
    if (!existing && !value) {
      setError("Paste the space's Google Chat webhook URL.");
      return;
    }
    if (value && !value.startsWith(WEBHOOK_PREFIX)) {
      setError(
        `That isn't a Google Chat webhook URL. It starts with ${WEBHOOK_PREFIX}`,
      );
      return;
    }
    const label =
      existing?.label ?? defaults[teamKey ?? ""]?.label ?? `${name} ${ladder}`;
    const what = routingWhat(name);
    drafts.add({
      family: familyOfLadder(ladder),
      what,
      kind: "room",
      roomId: existing?.id,
      url: value || undefined,
      input: {
        ladder,
        teamKey,
        label,
        enabled: existing?.enabled ?? true,
        teamless: teamKey ? teamless : (existing?.teamless ?? false),
        ...(teamKey ? { channel } : {}),
      },
    });
    onSaved(what);
    onClose();
  };

  return (
    <CaseAlertModal
      titleId={titleId}
      returnFocus={returnFocus}
      onClose={onClose}
    >
      <form className="cas-edit" noValidate onSubmit={submit}>
        <h3 id={titleId}>
          {room
            ? `Edit ${name.replace(/ \(shared space\)$/, "")} chat routing`
            : "Add team chat routing"}
        </h3>
        {room ? (
          <p className="cp-help">
            Team: <b>{name}</b>
          </p>
        ) : (
          <label>
            Team
            <select value={pick} onChange={(e) => pickTeam(e.target.value)}>
              {[...new Set(choices.map((c) => c.ladder))].map((l) => (
                <optgroup key={l} label={`${l} teams`}>
                  {choices
                    .filter((c) => c.ladder === l)
                    .map((c) => (
                      <option key={c.value} value={c.value}>
                        {c.room ? `${c.label} · has a space` : c.label}
                      </option>
                    ))}
                </optgroup>
              ))}
            </select>
          </label>
        )}
        <label>
          Google Chat webhook URL
          <input
            type="password"
            autoComplete="off"
            spellCheck={false}
            value={url}
            disabled={!webhooksEnabled}
            onChange={(e) => setUrl(e.target.value)}
            placeholder={`${WEBHOOK_PREFIX}v1/spaces/…`}
          />
        </label>
        <p className="cp-help">
          {existing?.hasWebhook
            ? `Leave empty to keep the saved URL (${existing.webhookHint ?? "hidden"}).`
            : "Copy it from the space: Apps & integrations, then Webhooks. It's never shown again."}
        </p>
        {teamKey && !roomsOnly ? (
          <label>
            Alerts by
            <select
              value={channel}
              onChange={(e) =>
                setChannel(e.target.value as CaseAlertChannel | "")
              }
            >
              <option value="">
                {ladderWords
                  ? `Same as the ladder (${ladderWords})`
                  : "Same as the ladder"}
              </option>
              {allowed.map((c) => (
                <option key={c} value={c}>
                  {channelWords(c)}
                </option>
              ))}
              {/* Saved before the ladder's channel narrowed: kept as is. */}
              {channel && !allowed.includes(channel) ? (
                <option value={channel}>
                  {channelWords(channel)} (more than the ladder)
                </option>
              ) : null}
            </select>
          </label>
        ) : null}
        {teamKey && ladder === "SRE" ? (
          <label className="cas-check">
            <input
              type="checkbox"
              checked={teamless}
              onChange={(e) => setTeamless(e.target.checked)}
            />
            Also gets incidents with no SRE team
          </label>
        ) : null}
        {error ? (
          <p className="cp-err" role="alert">
            {error}
          </p>
        ) : null}
        <p className="cas-sub cas-applied">
          Saved with this tab&apos;s Submit. {appliedNote}
        </p>
        <div className="cp-modal-actions">
          <button type="button" className="btn sm" onClick={onClose}>
            Cancel
          </button>
          {/* Kept with the tab's other changes until its Submit. */}
          <button type="submit" className="btn sm primary">
            Add to changes
          </button>
        </div>
      </form>
    </CaseAlertModal>
  );
}

interface SetupStepProps {
  settings: Settings;
  ladder: CaseAlertRoomLadder;
  onError: (msg: string | null) => void;
  onSaved: (what: string) => void;
}

/** The step left before moving off the file: the values the alert service
 *  runs with from its file that aren't saved here yet, saved in one request
 *  after a second click. Gone once there are none. */
function SetupStep({
  settings,
  ladder,
  onError,
  onSaved,
}: SetupStepProps): JSX.Element | null {
  const drafts = useCaseAlertDrafts();
  const [confirm, setConfirm] = useState(false);
  // Values already in this tab's changes are on their way: once queued (and
  // after Submit, once saved) they no longer count as the file's.
  const queued = new Set(
    drafts.changes.flatMap((c) =>
      c.kind === "values" && c.input.ladder === ladder ? Object.keys(c.input.values) : [],
    ),
  );
  const values = Object.fromEntries(
    Object.entries(valuesToCopy(settings, ladder)).filter(([k]) => !queued.has(k)),
  );
  const count = Object.keys(values).length;
  if (count === 0) return null;
  const word = count === 1 ? "setting" : "settings";

  return (
    <div className="cas-status warn cas-setup">
      <span>
        {count} {word} still {count === 1 ? "comes" : "come"} from the alert
        service&apos;s file.
      </span>
      {confirm ? (
        <>
          <span>
            Save {count} {word} here?
          </span>
          <button
            type="button"
            className="btn sm primary"
            disabled={drafts.submitting !== null}
            onClick={() => {
              onError(null);
              setConfirm(false);
              const what = `${count} ${ladder} ${word} from the file`;
              drafts.add({ family: familyOfLadder(ladder), what, kind: "values", input: { ladder, values } });
              onSaved(what);
            }}
          >
            Save {count} {word}
          </button>
          <button
            type="button"
            className="btn sm"
            onClick={() => setConfirm(false)}
          >
            Cancel
          </button>
        </>
      ) : (
        <button
          type="button"
          className="btn sm"
          disabled={drafts.submitting !== null}
          onClick={() => setConfirm(true)}
        >
          Save them here
        </button>
      )}
    </div>
  );
}

interface MoreMenuProps {
  disabled: boolean;
  items: { label: string; onSelect: () => void }[];
}

/** A row's less common actions behind one button. Opens a short list; Esc
 *  or a click outside closes it, and focus goes back to the button. Arrow
 *  keys move through the list. */
function MoreMenu({ disabled, items }: MoreMenuProps): JSX.Element {
  const [open, setOpen] = useState(false);
  const wrapRef = useRef<HTMLDivElement>(null);
  const buttonRef = useRef<HTMLButtonElement>(null);

  useEffect(() => {
    if (!open) return;
    wrapRef.current?.querySelector<HTMLElement>('[role="menuitem"]')?.focus();
    const onDown = (e: MouseEvent) => {
      if (!wrapRef.current?.contains(e.target as Node)) setOpen(false);
    };
    document.addEventListener("mousedown", onDown);
    return () => document.removeEventListener("mousedown", onDown);
  }, [open]);

  const onKeyDown = (e: KeyboardEvent<HTMLDivElement>) => {
    if (!open) return;
    if (e.key === "Escape") {
      e.stopPropagation();
      setOpen(false);
      buttonRef.current?.focus();
      return;
    }
    if (e.key !== "ArrowDown" && e.key !== "ArrowUp") return;
    e.preventDefault();
    const list = [
      ...(wrapRef.current?.querySelectorAll<HTMLElement>('[role="menuitem"]') ??
        []),
    ];
    const at = list.indexOf(document.activeElement as HTMLElement);
    const next =
      e.key === "ArrowDown"
        ? (at + 1) % list.length
        : (at - 1 + list.length) % list.length;
    list[next]?.focus();
  };

  return (
    <div className="cas-more" ref={wrapRef} onKeyDown={onKeyDown}>
      <button
        ref={buttonRef}
        type="button"
        className="btn sm"
        aria-haspopup="menu"
        aria-expanded={open}
        disabled={disabled}
        onClick={() => setOpen((o) => !o)}
      >
        More <span aria-hidden="true">▾</span>
      </button>
      {open ? (
        <ul className="cas-menu" role="menu">
          {items.map((item) => (
            <li key={item.label} role="none">
              <button
                type="button"
                role="menuitem"
                onClick={() => {
                  setOpen(false);
                  // Back to the button first; a question the item opens
                  // takes focus from there.
                  buttonRef.current?.focus();
                  item.onSelect();
                }}
              >
                {item.label}
              </button>
            </li>
          ))}
        </ul>
      ) : null}
    </div>
  );
}

interface RouteTableProps {
  ladder: CaseAlertLadder;
  routes: CaseAlertGroupRoute[];
  options?: CaseAlertRouteOptions;
  /** The ladder's saved rooms, to say where a team's alerts end up. */
  rooms: CaseAlertChatRoom[];
  /** The ladder's own channel from the file, when reported. */
  ladderChannel?: string;
  canEdit: boolean;
  /** When the alert service starts using a saved change (whenUsed). */
  appliedNote?: string;
  onError: (msg: string | null) => void;
  onSaved: (what: string) => void;
}

/** How each ladder's redirects are put to the reader. An SRE incident and a
 *  CRE case both carry the CSM Portal team they are assigned to; a redirect
 *  sends its alerts to one of the ladder's own teams instead. */
const ROUTE_WORDS: Record<
  CaseAlertLadder,
  { title: string; help: string; when: string; empty: string }
> = {
  SRE: {
    title: "Send an incident's alerts to another SRE team",
    help:
      "An incident is alerted to the SRE team it's assigned to in CSM Portal. If it's assigned to a different " +
      "team, such as an SME team, choose which SRE team should get its alerts: that team's chat space gets the " +
      "card, and its people on call get the calls, as set in Team chat routing above. An assignment group " +
      "that isn't a CSM Portal team (such as WSO2 IaaS Team) can be typed in under \"Another assignment group\".",
    when: "When an incident is assigned to",
    empty:
      "No redirects. Each incident is alerted to the SRE team it's assigned to.",
  },
  CRE: {
    title: "Send a case's alerts to another CRE team",
    help:
      "A case is alerted to the CRE team it's assigned to in CSM Portal. If a team there doesn't match a Team " +
      "Schedule team, choose which CRE team should get its alerts.",
    when: "When a case is assigned to",
    empty:
      "No redirects. Each case is alerted to the CRE team it's assigned to.",
  },
};

/** The CSM Portal teams a ladder's routes can be for: a CRE case's team, or
 *  an SRE incident's SRE or SME team. */
/** The group list's "type a name" entry: an SRE incident's assignment group
 *  that no CSM Portal team has. Not a name any team can have. */
const OTHER_GROUP = "\u0000other";

const ROUTE_GROUP_FAMILIES: Record<
  CaseAlertLadder,
  CaseAlertTeamOption["family"][]
> = {
  CRE: ["CRE"],
  SRE: ["SRE", "SME"],
};

function RouteTable({
  ladder,
  routes,
  options,
  rooms,
  ladderChannel,
  canEdit,
  appliedNote = DEFAULT_APPLIED_NOTE,
  onError,
  onSaved,
}: RouteTableProps): JSX.Element {
  const drafts = useCaseAlertDrafts();
  const family = familyOfLadder(ladder);
  const words = ROUTE_WORDS[ladder];
  const [group, setGroup] = useState("");
  // The typed name when group is OTHER_GROUP (SRE only).
  const [otherGroup, setOtherGroup] = useState("");
  const groupName = group === OTHER_GROUP ? otherGroup.trim() : group;
  // "team:<key>" or "rota:<code>": one list, the ladder's teams first.
  const [target, setTarget] = useState("");
  const [confirmId, setConfirmId] = useState<string | null>(null);
  const busy = drafts.submitting === family;
  const families = ROUTE_GROUP_FAMILIES[ladder];
  const groups = (options?.groups ?? []).filter((g) =>
    families.includes(g.family),
  );
  const teams = (options?.teams ?? []).filter((t) => t.family === ladder);
  const rotas = options?.rotas ?? [];
  // A saved route holds the team's name in lower case; show it as the team
  // is named, and a target by its name too.
  const groupLabel = (name: string) =>
    groups.find((g) => g.name.toLowerCase() === name.toLowerCase())?.name ??
    name;
  const teamLabel = (key: string) =>
    teams.find((t) => t.key === key)?.name ?? key;
  const rotaLabel = (code: string) => {
    const r = rotas.find((x) => x.code === code);
    return r && r.label !== code ? r.label : code;
  };
  /** Where a team's alerts end up, from Team chat routing: its chat space and
   *  how they go out. */
  const teamOutcome = (key: string) => {
    const room = rooms.find((r) => r.teamKey === key);
    const channel = room?.channel || ladderChannel;
    const by = channelWords(channel) ?? "As the ladder";
    // Where the card goes only matters when the team gets chat cards.
    if (channel === "call" || channel === "log") return by;
    const space = room?.hasWebhook
      ? `in the ${room.label} space`
      : "in its own chat space";
    return `${by} · ${space}`;
  };
  /** A redirect from a team to itself: it changes nothing. */
  // The alert service matches an incident's team by name against the Team
  // Schedule's key, so a redirect is only needless when the name already is
  // the key (a team named "Apollo" with key apollo). A team named "Apollo SRE
  // Group" with key apollo does need it, though both read "Apollo...".
  const sameTeam = (r: CaseAlertGroupRoute) =>
    !!r.teamKey && r.groupName.trim().toLowerCase() === r.teamKey.toLowerCase();

  const add = (e: FormEvent) => {
    e.preventDefault();
    onError(null);
    // Split at the first colon only: what follows is the key or code whole.
    const at = target.indexOf(":");
    if (at < 0) return;
    const kind = target.slice(0, at);
    const value = target.slice(at + 1);
    if (!groupName) return;
    const what = `redirect from ${groupLabel(groupName)} to ${
      kind === "team" ? teamLabel(value) : `every team on ${rotaLabel(value)}`
    }`;
    drafts.add({
      family,
      what,
      kind: "route",
      input: {
        ladder,
        groupName,
        ...(kind === "team" ? { teamKey: value } : { rotaCode: value }),
      },
    });
    onSaved(what);
    setGroup("");
    setOtherGroup("");
    setTarget("");
  };

  return (
    <>
      {routes.length === 0 ? (
        <p className="cas-sub cas-empty">{words.empty}</p>
      ) : (
        <table className="ra-tbl cas-routes">
          <thead>
            <tr>
              <th>Assigned to in CSM Portal</th>
              <th>Alerts go to</th>
              {canEdit ? <th aria-label="Actions" /> : null}
            </tr>
          </thead>
          <tbody>
            {routes.map((r) => (
              <tr key={r.id} className={r.enabled ? undefined : "cas-off"}>
                <td>{groupLabel(r.groupName)}</td>
                <td>
                  {r.teamKey ? (
                    <>
                      <b>{teamLabel(r.teamKey)}</b>
                      <div className="cas-sub">
                        {sameTeam(r)
                          ? "Same team, not needed: it is alerted as itself anyway"
                          : teamOutcome(r.teamKey)}
                      </div>
                    </>
                  ) : (
                    <>
                      <b>
                        Every team on the {rotaLabel(r.rotaCode ?? "")} rota
                      </b>
                      <div className="cas-sub">
                        Whoever is on call there gets the alerts
                      </div>
                    </>
                  )}
                </td>
                {canEdit ? (
                  <td>
                    <div className="cas-acts">
                      {confirmId === r.id ? (
                        <>
                          <button
                            type="button"
                            className="btn sm danger"
                            disabled={busy}
                            onClick={() => {
                              onError(null);
                              setConfirmId(null);
                              const what = `redirect from ${groupLabel(r.groupName)} removed`;
                              drafts.add({ family, what, kind: "deleteRoute", routeId: r.id });
                              onSaved(what);
                            }}
                          >
                            Remove
                          </button>
                          <button
                            type="button"
                            className="btn sm"
                            onClick={() => setConfirmId(null)}
                          >
                            Cancel
                          </button>
                        </>
                      ) : (
                        <button
                          type="button"
                          className="btn sm"
                          disabled={busy}
                          onClick={() => setConfirmId(r.id)}
                        >
                          Remove
                        </button>
                      )}
                    </div>
                  </td>
                ) : null}
              </tr>
            ))}
          </tbody>
        </table>
      )}
      {canEdit ? (
        <form className="cas-add cas-route-add" onSubmit={add}>
          <label>
            {words.when}
            <select value={group} onChange={(e) => setGroup(e.target.value)}>
              <option value="">Choose a team</option>
              {families.map((family) => {
                const inFamily = groups.filter((g) => g.family === family);
                return inFamily.length ? (
                  <optgroup key={family} label={`${family} teams`}>
                    {inFamily.map((g) => (
                      <option key={g.key} value={g.name}>
                        {g.name}
                      </option>
                    ))}
                  </optgroup>
                ) : null;
              })}
              {ladder === "SRE" ? (
                <optgroup label="Not a CSM Portal team">
                  <option value={OTHER_GROUP}>Another assignment group…</option>
                </optgroup>
              ) : null}
            </select>
          </label>
          {group === OTHER_GROUP ? (
            <label>
              Assignment group name
              <input
                type="text"
                value={otherGroup}
                maxLength={255}
                placeholder="As the incident carries it, e.g. WSO2 IaaS Team"
                onChange={(e) => setOtherGroup(e.target.value)}
              />
            </label>
          ) : null}
          <label>
            send its alerts to
            <select value={target} onChange={(e) => setTarget(e.target.value)}>
              <option value="">
                {ladder === "SRE" ? "Choose an SRE team" : "Choose a CRE team"}
              </option>
              <optgroup label={`${ladder} teams`}>
                {teams.map((t) => (
                  <option key={t.key} value={`team:${t.key}`}>
                    {t.name}
                  </option>
                ))}
              </optgroup>
              {ladder === "SRE" && rotas.length ? (
                <optgroup label="A whole rota">
                  {rotas.map((r) => (
                    <option key={r.code} value={`rota:${r.code}`}>
                      Every team on {rotaLabel(r.code)}
                    </option>
                  ))}
                </optgroup>
              ) : null}
            </select>
          </label>
          <button
            type="submit"
            className="btn sm"
            disabled={busy || !groupName || !target}
          >
            Add
          </button>
        </form>
      ) : null}
      {canEdit ? (
        <p className="cas-sub cas-applied">
          A redirect is saved with this tab&apos;s Submit. {appliedNote}
        </p>
      ) : null}
    </>
  );
}
