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

import { useMemo, useState, type JSX, type ReactNode } from "react";
import QueryErrorState from "@components/QueryErrorState";
import { useErrorBanner } from "@context/error-banner/ErrorBannerContext";
import { BackendApiError } from "@api/backend/client";
import { useGetPagingChain, usePatchPagingMember } from "../api/usePagingChain";
import type {
  PagingChainMember,
  PagingMemberChange,
  PagingReadinessChain,
  PagingReadinessGap,
} from "../types";
import { mayBeResponder, responderNeedsProfilePhoneMessage } from "../utils/pagingPhone";
import { teamDisplayName } from "../utils/teamDisplayName";
import PagingReadinessStrip, { type PhoneGapAction } from "./PagingReadinessStrip";
import PhoneChip, { type PagingPhoneControls } from "./PhoneChip";
import ScheduleSkeleton from "./ScheduleSkeleton";

/**
 * The Case Paging tab: who the system pages about an incident, tier by tier,
 * until someone acknowledges. Drawn as a matrix: a column per team (the ABTs,
 * then Americas, the night cover) and a row per level -- the rules sheet's
 * LEVEL_0 to LEVEL_4, which the paging configuration calls Tiers 1 to 5.
 *
 *   Level 0 (L0)   each team's 1st responder; 2nd and 3rd are optional
 *   Level 1 (L1)   the team's Team lead (at night, Americas' Team leads)
 *   Level 2 (L2)   the Team leads pool by day; at night the America lead
 *   Level 3 (L3)   CRE head
 *   Level 4 (L4)   CS head
 *
 * Every person is an existing team membership. What the reader may change
 * comes from the server (`canEdit`), which checks every edit again; a locked
 * picker here only reflects that.
 *
 * This is the CRE chain, the one whose people are set here. The SRE and SME
 * chains take theirs from the rota, so the tab shows them read-only.
 */

/** The family whose alert chain this panel edits. */
export const PAGING_PANEL_FAMILY = "CRE";

interface Props {
  /** The tab's chain picker, drawn first in the card head. */
  chainPicker?: ReactNode;
  /** The readiness strip's read for this chain. */
  readiness: {
    chain?: PagingReadinessChain;
    days: number;
    isLoading: boolean;
    isError: boolean;
  };
  /** A rota gap's action: open the week view on that rota. */
  onOpenRota?: (gap: PagingReadinessGap) => void;
  /** The paging-number actions; the chips beside each person show the number
   *  paging would call either way. */
  phone?: PagingPhoneControls;
  /** A readiness phone gap's inline action. */
  phoneActionFor?: (gap: PagingReadinessGap) => PhoneGapAction | null;
}

interface TeamGroup {
  key: string;
  name: string;
  type: string;
  members: PagingChainMember[];
}

const RANKS = [1, 2, 3] as const;
const RANK_LABEL: Record<number, string> = { 1: "1st responder", 2: "2nd responder", 3: "3rd responder" };

const isLeadership = (t: TeamGroup) =>
  t.type.toLowerCase().includes("leadership") ||
  t.members.some((m) => m.role === "cre_head" || m.role === "cs_head");
const isABT = (t: TeamGroup) => t.type.toLowerCase().endsWith("abt");
const canRespond = (m: PagingChainMember) => m.role === "engineer" || m.role === "sub_lead";
/** The Level 0 (L0) 1st-responder cell of a team's column: where "Set
 *  responders" lands. */
const firstResponderCellId = (teamKey: string) => `cp-r1-${teamKey}`;
/** A row's label, by the rules sheet's level: "Level 0 (L0)". */
const levelLabel = (level: number) => `Level ${level} (L${level})`;

/** What the banner says when a change is refused: the server's own reason
 *  where it gave one the reader can act on, and that nothing else moved. */
function notChanged(err: unknown): string {
  const reason =
    err instanceof BackendApiError && (err.status === 403 || err.status === 409) && err.message
      ? ` ${err.message.charAt(0).toUpperCase()}${err.message.slice(1)}.`
      : "";
  return `That change to the alert chain was not saved.${reason}`;
}

function LockHint({ who }: { who: string }): JSX.Element {
  return (
    <span className="cp-lock" title={`Set by ${who}`}>
      <svg width="11" height="11" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.4" aria-hidden="true">
        <rect x="4" y="11" width="16" height="10" rx="2" />
        <path d="M8 11V7a4 4 0 0 1 8 0v4" />
      </svg>
      {who}
    </span>
  );
}

export default function CasePagingPanel({
  chainPicker,
  readiness,
  onOpenRota,
  phone,
  phoneActionFor,
}: Props): JSX.Element {
  const family = PAGING_PANEL_FAMILY;
  const chain = useGetPagingChain(family);
  const patch = usePatchPagingMember();
  const { showError } = useErrorBanner();
  const [editing, setEditing] = useState(false);
  const [teamFilter, setTeamFilter] = useState("");
  const [changed, setChanged] = useState<Set<string>>(new Set());

  const teams = useMemo<TeamGroup[]>(() => {
    const byKey = new Map<string, TeamGroup>();
    for (const m of chain.data?.members ?? []) {
      let t = byKey.get(m.teamKey);
      if (!t) {
        t = { key: m.teamKey, name: teamDisplayName(m.teamName || m.teamKey), type: m.teamType, members: [] };
        byKey.set(m.teamKey, t);
      }
      t.members.push(m);
    }
    return [...byKey.values()];
  }, [chain.data?.members]);

  const can = chain.data?.canEdit;
  const leadership = teams.find(isLeadership);
  const chainTeams = teams.filter((t) => t !== leadership);
  const shownTeams = teamFilter ? chainTeams.filter((t) => t.key === teamFilter) : chainTeams;

  /** Whether the reader may change anything on this chain. A reader who may
   *  not (an SME or SRE rota admin, say) is not offered "Edit alert chain"
   *  at all; the server still checks every edit. */
  const mayEditAny =
    !!can &&
    (can.responderTeams.length > 0 || can.teamLeadTeams.length > 0 || can.americasTeamLead || can.heads);
  const mayResponders = (key: string) => !!can?.responderTeams.includes(key.toLowerCase());
  const mayTeamLeads = (key: string) => !!can?.teamLeadTeams.includes(key.toLowerCase());

  /** Run one or more changes in order; one failing stops the rest. */
  const apply = async (changes: PagingMemberChange[], cell: string) => {
    try {
      for (const c of changes) {
        await patch.mutateAsync(c);
      }
      setChanged((prev) => new Set(prev).add(cell));
    } catch (err) {
      showError(notChanged(err), err);
    }
  };

  /** A readiness gap's "Set responders": the team's column, its 1st
   *  responder cell in view and focused, and editing on where the reader may
   *  set its responders. */
  const fixResponders = (teamKey?: string) => {
    setTeamFilter("");
    const key = teamKey ? chainTeams.find((t) => t.key.toLowerCase() === teamKey.toLowerCase())?.key : undefined;
    if (!editing && (key ? mayResponders(key) : (can?.responderTeams.length ?? 0) > 0)) {
      setEditing(true);
      setChanged(new Set());
    }
    if (!key) return;
    // After the cell has rendered with the filter cleared and editing on.
    requestAnimationFrame(() => {
      const cell = document.getElementById(firstResponderCellId(key));
      if (!cell) return;
      cell.scrollIntoView?.({ block: "center", inline: "center" });
      (cell.querySelector<HTMLSelectElement>("select:not(:disabled)") ?? cell).focus();
    });
  };

  const gapsOf = (t: TeamGroup): string[] => {
    const gaps: string[] = [];
    const ranks = new Set(t.members.map((m) => m.responderRank).filter((r) => r > 0));
    const missing = RANKS.filter((r) => !ranks.has(r));
    if (missing.length) gaps.push(`${missing.map((r) => RANK_LABEL[r]).join(", ")} empty`);
    const leads = t.members.filter((m) => m.role === "lead");
    if (isABT(t) && leads.length === 0) gaps.push("No Team lead");
    if (!isABT(t) && !t.members.some((m) => m.role === "americas_team_lead")) gaps.push("No America lead");
    return gaps;
  };
  const totalGaps = chainTeams.reduce((n, t) => n + gapsOf(t).length, 0);

  const head = (role: "cre_head" | "cs_head") => leadership?.members.find((m) => m.role === role);

  const nameOf = (m?: PagingChainMember) => (m ? m.name || m.email : undefined);

  /** One person's cell: the picker while editing (locked where the reader may
   *  not change it, saying who may), the name otherwise; and the number
   *  paging would call them on. */
  const personCell = (o: {
    label: string;
    cellKey: string;
    holder?: PagingChainMember;
    options: PagingChainMember[];
    allowed: boolean;
    onPick: (membershipId: string) => void;
    lockWho?: string;
    rank?: number;
  }): JSX.Element => (
    <>
      {editing ? (
        <select
          className={`cp-pick${changed.has(o.cellKey) ? " chg" : ""}`}
          aria-label={o.label}
          data-rank={o.rank}
          disabled={!o.allowed}
          value={o.holder?.membershipId ?? ""}
          onChange={(e) => o.onPick(e.target.value)}
        >
          <option value="">— not set —</option>
          {o.options.map((m) => (
            <option key={m.membershipId} value={m.membershipId}>
              {nameOf(m)}
            </option>
          ))}
        </select>
      ) : (
        <span className={`cp-name${o.holder ? "" : " unset"}`}>{nameOf(o.holder) ?? "— not set —"}</span>
      )}
      {o.holder ? <PhoneChip member={o.holder} controls={phone} /> : null}
      {editing && !o.allowed && o.lockWho ? <LockHint who={o.lockWho} /> : null}
    </>
  );

  const headCell = (role: "cre_head" | "cs_head", label: string) =>
    personCell({
      label,
      cellKey: role,
      holder: head(role),
      options: leadership?.members ?? [],
      allowed: !!can?.heads,
      onPick: (id) => {
        if (id) void apply([{ membershipId: id, role }], role);
      },
      lockWho: "CRE rota admin, portal admin",
    });

  // The columns: the ABTs in the server's order, then the night cover.
  const abtTeams = shownTeams.filter(isABT);
  const nightTeams = shownTeams.filter((t) => !isABT(t));
  const columns = [...abtTeams, ...nightTeams];
  const colClass = (t: TeamGroup) => (isABT(t) ? undefined : "am");

  return (
    <>
      <div className="card-head">
        {chainPicker}
        <h2>
          Alert chain <span className="count">{chainTeams.length} teams</span>
        </h2>
        <div className="tools">
          {totalGaps > 0 ? <span className="rng">{totalGaps} gap{totalGaps === 1 ? "" : "s"}</span> : null}
          {mayEditAny || editing ? (
            <button
              className={`btn sm${editing ? " primary" : ""}`}
              aria-pressed={editing}
              onClick={() => {
                setEditing((v) => !v);
                setChanged(new Set());
              }}
            >
              <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" aria-hidden="true">
                <path d="M4 20h4l10-10a2.8 2.8 0 0 0-4-4L4 16v4z" />
              </svg>
              <span>{editing ? "Done editing" : "Edit alert chain"}</span>
            </button>
          ) : null}
          <select
            className="teampick"
            aria-label="Show one team"
            value={teamFilter}
            onChange={(e) => setTeamFilter(e.target.value)}
          >
            <option value="">All teams</option>
            {chainTeams.map((t) => (
              <option key={t.key} value={t.key}>
                {t.name}
              </option>
            ))}
          </select>
        </div>
      </div>

      {editing ? (
        <div className="editbar">
          <span className="pill">Editing</span>
          <span className="hintx">pick a person in any unlocked cell · changes save as you make them</span>
        </div>
      ) : null}

      <PagingReadinessStrip
        label="CRE"
        {...readiness}
        onFixResponders={fixResponders}
        onOpenRota={onOpenRota}
        phoneActionFor={phoneActionFor}
      />

      {chain.isError ? (
        <QueryErrorState message="Could not load the alert chain." error={chain.error} />
      ) : chain.isLoading ? (
        <ScheduleSkeleton variant="list" label="Loading the alert chain…" rows={4} />
      ) : chainTeams.length === 0 ? (
        <div className="offnone">No teams on the {family} alert chain.</div>
      ) : (
        <div className="cp-mx-wrap">
          <table className="cp-mx">
            <caption className="cp-vh">CRE alert chain by team and level</caption>
            <thead>
              <tr>
                <th scope="col" className="cp-corner">
                  Level
                </th>
                {columns.map((t) => {
                  const gaps = gapsOf(t);
                  return (
                    <th key={t.key} scope="col" className={colClass(t)}>
                      <span className="cp-tn">{t.name}</span>
                      {isABT(t) ? null : <small>night</small>}
                      <span className={`cp-st ${gaps.length === 0 ? "ok" : "gap"}`} title={gaps.join(" · ") || undefined}>
                        {gaps.length === 0 ? "Ready" : `${gaps.length} gap${gaps.length === 1 ? "" : "s"}`}
                        {gaps.length > 0 ? <span className="cp-vh">: {gaps.join("; ")}</span> : null}
                      </span>
                    </th>
                  );
                })}
              </tr>
            </thead>

            {/* L0: the responders, paged first, all together -- one row, its
                cell in each column split into the 1st, 2nd and 3rd. */}
            <tbody className="band">
              <tr>
                <th scope="row">
                  {levelLabel(0)}
                  <small>
                    1st, 2nd, 3rd responders
                    <span className="cp-opt"> · 2nd and 3rd optional</span>
                  </small>
                </th>
                {columns.map((t) => {
                  const abt = isABT(t);
                  return (
                    <td key={t.key} className={colClass(t)} id={firstResponderCellId(t.key)} tabIndex={-1}>
                      <ol className="cp-l0" aria-label={`${t.name} responders`}>
                        {RANKS.map((r) => (
                          <li key={r} className="cp-slot">
                            <span className="cp-rank">
                              {RANK_LABEL[r].replace(" responder", "")}
                              {r > 1 ? <span className="cp-opt"> · optional</span> : null}
                            </span>
                            {personCell({
                              label: `${t.name} ${RANK_LABEL[r]}`,
                              cellKey: `${t.key}|r${r}`,
                              rank: r,
                              holder: t.members.find((m) => m.responderRank === r),
                              options: t.members.filter(canRespond),
                              allowed: mayResponders(t.key),
                              onPick: (next) => {
                                const holder = t.members.find((m) => m.responderRank === r);
                                // A responder must be reachable: someone with
                                // no number on their profile is refused here,
                                // before asking (the backend refuses it too).
                                const picked = next ? t.members.find((m) => m.membershipId === next) : undefined;
                                if (picked && !mayBeResponder(picked)) {
                                  showError(`That change to the alert chain was not saved. ${responderNeedsProfilePhoneMessage(nameOf(picked) ?? "This person")}.`);
                                  return;
                                }
                                void apply(
                                  next
                                    ? [{ membershipId: next, responderRank: r }]
                                    : holder
                                      ? [{ membershipId: holder.membershipId, responderRank: 0 }]
                                      : [],
                                  `${t.key}|r${r}`,
                                );
                              },
                              lockWho:
                                r === 1 ? (abt ? "this team's lead, rota admin" : "America lead, rota admin") : undefined,
                            })}
                          </li>
                        ))}
                      </ol>
                    </td>
                  );
                })}
              </tr>
            </tbody>

            {/* L1: each ABT's Team lead; at night, Americas' Team leads. */}
            <tbody>
              <tr>
                <th scope="row">
                  {levelLabel(1)}
                  <small>Team lead</small>
                </th>
                {columns.map((t) => {
                  const leads = t.members.filter((m) => m.role === "lead");
                  if (isABT(t)) {
                    return (
                      <td key={t.key}>
                        {personCell({
                          label: `${t.name} Team lead`,
                          cellKey: `${t.key}|lead`,
                          holder: leads[0],
                          options: t.members,
                          allowed: mayTeamLeads(t.key),
                          onPick: (next) => {
                            if (!next) return;
                            void apply(
                              [
                                { membershipId: next, role: "lead" },
                                ...leads
                                  .filter((l) => l.membershipId !== next)
                                  .map((l) => ({ membershipId: l.membershipId, role: "engineer" as const })),
                              ],
                              `${t.key}|lead`,
                            );
                          },
                          lockWho: "CRE/CS head, rota admin",
                        })}
                      </td>
                    );
                  }
                  const okLeads = editing && mayTeamLeads(t.key);
                  return (
                    <td key={t.key} className="am">
                      {/* A compact list -- a lead a line, the number inline --
                          so a long night roster does not make this the
                          tallest row. Each lead's number actions (Test call)
                          are offered as in every other cell, editing or not. */}
                      <div className="cp-leadcell">
                        {leads.length > 0 ? (
                          <ul className="cp-leads" aria-label={`${t.name} Team leads`}>
                            {leads.map((m) => (
                              <li key={m.membershipId}>
                                <span className="cp-lname">{nameOf(m)}</span>
                                <PhoneChip member={m} controls={phone} />
                                {okLeads ? (
                                  <button
                                    type="button"
                                    className="cp-x"
                                    aria-label={`Remove ${nameOf(m)} as a Team lead`}
                                    onClick={() =>
                                      void apply([{ membershipId: m.membershipId, role: "engineer" }], `${t.key}|leads`)
                                    }
                                  >
                                    ×
                                  </button>
                                ) : null}
                              </li>
                            ))}
                          </ul>
                        ) : (
                          <span className="cp-note">No Team leads yet</span>
                        )}
                        {okLeads ? (
                          <select
                            className="cp-pick cp-add"
                            aria-label={`Add a ${t.name} Team lead`}
                            value=""
                            onChange={(e) =>
                              e.target.value &&
                              void apply([{ membershipId: e.target.value, role: "lead" }], `${t.key}|leads`)
                            }
                          >
                            <option value="">+ Add Team lead</option>
                            {t.members.filter(canRespond).map((m) => (
                              <option key={m.membershipId} value={m.membershipId}>
                                {nameOf(m)}
                              </option>
                            ))}
                          </select>
                        ) : editing ? (
                          <LockHint who="America lead, rota admin" />
                        ) : null}
                      </div>
                    </td>
                  );
                })}
              </tr>
            </tbody>

            {/* L2: by day the ABT Team leads pool; at night the America lead. */}
            <tbody className="band">
              <tr>
                <th scope="row">
                  {levelLabel(2)}
                  <small>Team leads pool · America lead at night</small>
                </th>
                {abtTeams.length > 0 ? (
                  <td colSpan={abtTeams.length} className="cp-pool">
                    <span className="cp-name">Team leads pool</span>
                    <div className="cp-note">The ABT Team leads above, longest since called first</div>
                    <div className="cp-note">How many it calls is set in the alert configuration, not here.</div>
                  </td>
                ) : null}
                {nightTeams.map((t) => {
                  const leads = t.members.filter((m) => m.role === "lead");
                  const teamHead = t.members.find((m) => m.role === "americas_team_lead");
                  return (
                    <td key={t.key} className="am">
                      {personCell({
                        label: `${t.name} America lead`,
                        cellKey: `${t.key}|head`,
                        holder: teamHead,
                        options: [...(teamHead ? [teamHead] : []), ...leads],
                        allowed: !!can?.americasTeamLead,
                        onPick: (next) => {
                          if (next) void apply([{ membershipId: next, role: "americas_team_lead" }], `${t.key}|head`);
                        },
                        lockWho: "rota admin, portal admin",
                      })}
                    </td>
                  );
                })}
              </tr>
            </tbody>

            {/* L3 and L4: above every team. */}
            <tbody>
              <tr>
                <th scope="row">
                  {levelLabel(3)}
                  <small>CRE head</small>
                </th>
                <td colSpan={columns.length} className="cp-span">
                  {headCell("cre_head", "CRE head")}
                </td>
              </tr>
            </tbody>
            <tbody className="band">
              <tr>
                <th scope="row">
                  {levelLabel(4)}
                  <small>CS head</small>
                </th>
                <td colSpan={columns.length} className="cp-span">
                  {headCell("cs_head", "CS head")}
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      )}
    </>
  );
}
