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
import { useSearchInternalUsersByName } from "@api/useSearchUsersByName";
import { useDebouncedValue } from "@hooks/useDebouncedValue";
import { useChangeRotaAccess, useRotaAccess, type RotaAccessChange } from "../api/useRotaAccess";
import type { RotaAdminGrant, RotaFamily, RotaMarkerGrant, ScheduleTeam } from "../types";
import RotaReminderSettings from "./RotaReminderSettings";
import ScheduleSkeleton from "./ScheduleSkeleton";

const FAMILIES: RotaFamily[] = ["CRE", "SRE", "SME"];

interface Props {
  /** Every team on the schedule, from the catalogue: the marker lists are
   *  drawn per team, and a marker is picked from the team's own members. */
  teams: ScheduleTeam[];
}

/**
 * The Rota access tab: who may mark the rotas besides each team's lead.
 *
 * Two grants, both granted and taken back here:
 *   Rota admin   marks every team in one group (CRE, SRE or SME). Only a CSM
 *                admin grants it.
 *   Rota marker  marks one team -- its rotations, leave and allocations -- and
 *                nothing else: not Generate month, not the tags, not the
 *                paging chain. A CSM or portal admin names any team's, a
 *                rota admin their own group's, a team's lead their own
 *                team's, and only from the team's own members.
 *
 * The server decides who may change what and answers with it (canManage); the
 * page only offers what it says. Every change saves the moment it is made.
 */
export default function RotaAccessTab({ teams }: Props): JSX.Element {
  const access = useRotaAccess();
  const change = useChangeRotaAccess();
  const [error, setError] = useState<string | null>(null);

  const canAdmins = access.data?.canManage.rotaAdmins ?? false;
  const markerFamilies = useMemo(() => access.data?.canManage.markerFamilies ?? [], [access.data]);
  // The teams the reader leads: their Rota Managers are theirs to change even
  // in a group the reader does not run.
  const markerTeams = useMemo(
    () => new Set((access.data?.canManage.markerTeams ?? []).map((k) => k.toLowerCase())),
    [access.data],
  );
  const ledHere = (t: ScheduleTeam): boolean => markerTeams.has(t.key.toLowerCase());
  // Every group the reader may change some team of, in the usual order.
  const groups = FAMILIES.filter((f) => markerFamilies.includes(f) || teams.some((t) => t.family === f && ledHere(t)));
  const [familyChoice, setFamily] = useState<RotaFamily | null>(null);
  const family = familyChoice && groups.includes(familyChoice) ? familyChoice : groups[0];

  const apply = (c: RotaAccessChange): void => {
    setError(null);
    change.mutate(c, { onError: (e) => setError(e.message || "That change did not save.") });
  };

  if (access.isLoading) return <ScheduleSkeleton variant="list" label="Loading rota access…" rows={5} />;
  if (access.isError || !access.data) return <div className="offnone">Rota Access could not be loaded.</div>;

  return (
    <div className="ra">
      <div className="card-head">
        <h2>Rota Access</h2>
      </div>
      <p className="ra-intro">
        A team&apos;s lead always marks their own team. Here you can let others do it too. A{" "}
        <b>Rota admin</b> marks every team in one group. A <b>Rota Manager</b> marks one team&apos;s rotations,
        leave and allocations, but can&apos;t generate the month, change the tags or the alert chain.
      </p>
      {error ? (
        <div className="ra-err" role="alert">
          {error}
        </div>
      ) : null}

      {canAdmins ? (
        <section className="ra-sec" aria-labelledby="ra-admins-h">
          <h3 id="ra-admins-h">Rota admins</h3>
          <div className="ra-fams">
            {FAMILIES.map((f) => (
              <AdminColumn
                key={f}
                family={f}
                holders={access.data.rotaAdmins.filter((a) => a.family === f)}
                busy={change.isPending}
                onChange={apply}
              />
            ))}
          </div>
        </section>
      ) : null}

      {family ? (
        <section className="ra-sec" aria-labelledby="ra-markers-h">
          <div className="ra-sech">
            <h3 id="ra-markers-h">Rota Managers</h3>
            {groups.length > 1 ? (
              <div className="seg teamseg" role="tablist" aria-label="Group">
                {groups.map((f) => (
                  <button
                    key={f}
                    id={`ra-fam-${f}`}
                    type="button"
                    role="tab"
                    aria-selected={family === f}
                    aria-controls="ra-markers-panel"
                    className={family === f ? "on" : undefined}
                    onClick={() => setFamily(f)}
                  >
                    {f}
                  </button>
                ))}
              </div>
            ) : null}
          </div>
          {/* One panel for the group chosen above; with a single group there
              are no tabs, so it is a plain region named by the heading. */}
          <div
            id="ra-markers-panel"
            role={groups.length > 1 ? "tabpanel" : undefined}
            aria-labelledby={groups.length > 1 ? `ra-fam-${family}` : undefined}
          >
            <MarkerTable
              teams={teams.filter((t) => t.family === family && (markerFamilies.includes(family) || ledHere(t)))}
              markers={access.data.markers.filter((m) => m.family === family)}
              busy={change.isPending}
              onChange={apply}
            />
          </div>
        </section>
      ) : null}

      {markerFamilies.includes("CRE") ? <RotaReminderSettings /> : null}
    </div>
  );
}

/** When and by whom, for a grant's tooltip. */
function grantedNote(g: { grantedBy: string; grantedOn: string; active: boolean }, inactive: string): string {
  const on = g.grantedOn ? new Date(g.grantedOn).toLocaleDateString() : "";
  const by = g.grantedBy ? ` by ${g.grantedBy}` : "";
  return `${g.active ? "" : `${inactive} `}Granted${by}${on ? ` on ${on}` : ""}`.trim();
}

/** One group's rota admins: who holds it, removing one, and adding somebody. */
function AdminColumn({
  family,
  holders,
  busy,
  onChange,
}: {
  family: RotaFamily;
  holders: RotaAdminGrant[];
  busy: boolean;
  onChange: (c: RotaAccessChange) => void;
}): JSX.Element {
  return (
    <div className="ra-fam">
      <div className="ra-famh">
        {family} rota admins <b>{holders.length}</b>
      </div>
      {holders.length > 0 ? (
        <ul className="ra-list" aria-label={`${family} rota admins`}>
          {holders.map((a) => (
            <li key={a.userId} className={a.active ? undefined : "off"}>
              <span className="ra-name" title={grantedNote(a, "Not internal staff, so this does nothing.")}>
                {a.name || a.email}
              </span>
              {a.active ? null : <span className="ra-tag">inactive</span>}
              <button
                type="button"
                className="cp-x"
                disabled={busy}
                aria-label={`Remove ${a.name || a.email} as ${family} rota admin`}
                onClick={() => onChange({ kind: "admin", family, userId: a.userId, grant: false })}
              >
                ×
              </button>
            </li>
          ))}
        </ul>
      ) : (
        <span className="cp-note">No {family} rota admins</span>
      )}
      <PersonSearch
        label={`Add a ${family} rota admin`}
        exclude={holders.map((h) => h.userId)}
        busy={busy}
        onPick={(userId) => onChange({ kind: "admin", family, userId, grant: true })}
      />
    </div>
  );
}

/**
 * Internal staff, for a rota admin -- who need not be on any team (a manager
 * running a group's rotas often is not). The directory search matches the
 * username and email, not the display name, hence "name or email".
 */
function PersonSearch({
  label,
  exclude,
  busy,
  onPick,
}: {
  label: string;
  exclude: string[];
  busy: boolean;
  onPick: (userId: string) => void;
}): JSX.Element {
  const [text, setText] = useState("");
  const q = useDebouncedValue(text.trim(), 250);
  const found = useSearchInternalUsersByName(q, q.length >= 2);
  const people = (found.data ?? []).filter((u) => u.id && !exclude.includes(u.id));
  const nameOf = (u: { firstName?: string; lastName?: string; email?: string }) =>
    [u.firstName, u.lastName].filter(Boolean).join(" ") || u.email || "";

  return (
    <div className="ra-search">
      <input
        type="search"
        className="ra-input"
        placeholder="+ Add by name or email"
        aria-label={label}
        value={text}
        disabled={busy}
        onChange={(e) => setText(e.target.value)}
      />
      {q.length >= 2 ? (
        <ul className="ra-found" aria-label={`${label}: matches`}>
          {found.isLoading ? <li className="cp-note">Searching…</li> : null}
          {!found.isLoading && people.length === 0 ? <li className="cp-note">Nobody found</li> : null}
          {people.map((u) => (
            <li key={u.id}>
              <button
                type="button"
                disabled={busy}
                onClick={() => {
                  onPick(u.id as string);
                  setText("");
                }}
              >
                <span className="ra-name">{nameOf(u)}</span>
                {u.email ? <span className="ra-mail">{u.email}</span> : null}
              </button>
            </li>
          ))}
        </ul>
      ) : null}
    </div>
  );
}

/** One group's teams, each with its rota markers and a pick from its members. */
function MarkerTable({
  teams,
  markers,
  busy,
  onChange,
}: {
  teams: ScheduleTeam[];
  markers: RotaMarkerGrant[];
  busy: boolean;
  onChange: (c: RotaAccessChange) => void;
}): JSX.Element {
  const byTeam = useMemo(() => {
    const out = new Map<string, RotaMarkerGrant[]>();
    for (const m of markers) out.set(m.teamKey, [...(out.get(m.teamKey) ?? []), m]);
    return out;
  }, [markers]);
  const sorted = useMemo(() => [...teams].sort((a, b) => a.sortOrder - b.sortOrder), [teams]);

  if (sorted.length === 0) return <div className="offnone">No teams in this group.</div>;
  return (
    <table className="ra-tbl">
      <thead>
        <tr>
          <th scope="col">Team</th>
          <th scope="col">Lead</th>
          <th scope="col">Rota Managers</th>
        </tr>
      </thead>
      <tbody>
        {sorted.map((t) => {
          const held = byTeam.get(t.key) ?? [];
          const heldIds = new Set(held.map((m) => m.userId));
          const members = t.members ?? [];
          const leads = members.filter((m) => m.isLead);
          // A lead already marks their own team; offering them would grant
          // nothing they do not hold.
          const pickable = members.filter((m) => !m.isLead && !heldIds.has(m.userId));
          return (
            <tr key={t.key}>
              <th scope="row" className="ra-team">
                {t.name}
              </th>
              <td className="ra-leads">{leads.length > 0 ? leads.map((m) => m.name).join(", ") : "—"}</td>
              <td>
                <div className="ra-cell">
                  {held.length > 0 ? (
                    <ul className="ra-list" aria-label={`${t.name} Rota Managers`}>
                      {held.map((m) => (
                        <li key={m.userId} className={m.active ? undefined : "off"}>
                          <span
                            className="ra-name"
                            title={grantedNote(m, "No longer on the team or not internal staff, so this does nothing.")}
                          >
                            {m.name || m.email}
                          </span>
                          {m.active ? null : <span className="ra-tag">inactive</span>}
                          <button
                            type="button"
                            className="cp-x"
                            disabled={busy}
                            aria-label={`Remove ${m.name || m.email} as ${t.name} Rota Manager`}
                            onClick={() => onChange({ kind: "marker", teamKey: t.key, userId: m.userId, grant: false })}
                          >
                            ×
                          </button>
                        </li>
                      ))}
                    </ul>
                  ) : null}
                  <select
                    className="cp-pick cp-add"
                    aria-label={`Add Rota Manager for ${t.name}`}
                    value=""
                    disabled={busy || pickable.length === 0}
                    onChange={(e) =>
                      e.target.value && onChange({ kind: "marker", teamKey: t.key, userId: e.target.value, grant: true })
                    }
                  >
                    <option value="">{pickable.length === 0 ? "No one else on the team" : "+ Add Rota Manager"}</option>
                    {pickable.map((m) => (
                      <option key={m.userId} value={m.userId}>
                        {m.name}
                      </option>
                    ))}
                  </select>
                </div>
              </td>
            </tr>
          );
        })}
      </tbody>
    </table>
  );
}
