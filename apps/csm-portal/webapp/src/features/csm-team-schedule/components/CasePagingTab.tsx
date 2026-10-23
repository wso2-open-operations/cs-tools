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

import { useState, type JSX } from "react";
import { useCaseAlertSettings } from "../api/useCaseAlertSettings";
import {
  useGetPagingChain,
  useGetPagingReadiness,
} from "../api/usePagingChain";
import type {
  CaseAlertSettings as AlertSettingsData,
  PagingChainCode,
  PagingChainMember,
  PagingReadinessGap,
  RotaFamily,
  ScheduleTeam,
} from "../types";
import { mayManageChain, mayManageFamily } from "../utils/caseAlertChainSettings";
import type { SettingsFamily } from "../utils/caseAlertDrafts";
import type { CaseAlertChain } from "../utils/pagingChains";
import CaseAlertDraftsProvider from "./CaseAlertDraftsProvider";
import CaseAlertSettings from "./CaseAlertSettings";
import CasePagingPanel from "./CasePagingPanel";
import PagingReadinessStrip, {
  type PhoneGapAction,
} from "./PagingReadinessStrip";
import { CALLING_LABEL } from "./PhoneChip";
import { usePagingPhoneControls } from "./usePagingPhoneControls";

/** The readiness gaps that are about one person's number. */
const PHONE_GAPS = new Set(["NO_PHONE", "PHONE_UNTESTED", "PHONE_TEST_FAILED"]);

/** What a family's tab shows: who gets its alerts, or its settings. */
type ChainView = "chain" | "settings";

/** Where the last choice is kept, for this browser. */
const VIEW_KEY = "csm-ts.alert-settings.view";

/** The last choice, else who gets alerts. Storage can be blocked or throw
 *  (private windows, strict settings): then it is just the default. */
function readView(): ChainView {
  try {
    return window.localStorage.getItem(VIEW_KEY) === "settings"
      ? "settings"
      : "chain";
  } catch {
    return "chain";
  }
}

function writeView(view: ChainView): void {
  try {
    window.localStorage.setItem(VIEW_KEY, view);
  } catch {
    // Not kept: the choice still holds until the page is left.
  }
}

/** How far ahead the readiness strip looks. */
export const PAGING_READINESS_DAYS = 7;

/** Where a chain's people come from on the rota: the family, and the rota
 *  within it where the chain is one rota's (SME draws on every SME rota). */
export interface PagingRotaTarget {
  family: RotaFamily;
  rotaCode?: string;
  teamKey?: string;
  zoneCode?: string;
  /** YYYY-MM-DD. */
  date?: string;
}

interface ChainOption {
  code: CaseAlertChain;
  label: string;
  /** Where its people are set, for a chain that takes them from the rota. */
  rota?: { family: RotaFamily; rotaCode?: string };
  disabledReason?: string;
  /** A chain named after an assignment group (U2-SRE, B-Central-SRE): shown
   *  only once its team is an SRE team in the catalogue, which entity-service
   *  migration 0242 makes it. Before that the team is SME and its readiness is
   *  on the SME chain. */
  sreTeamKey?: string;
}

/** The chains under the SRE tab's "Who gets alerts", in order: the SRE
 *  sub-teams (SaaS, IaaS, then U2-SRE and B-Central-SRE, named after their
 *  assignment groups), PaaS, then SME. PaaS SRE has no rota yet, so it is listed -- it is a
 *  chain people expect -- with no chain to show. CRE's "Who gets alerts" is
 *  its own editable chain (CasePagingPanel), where the CRE Team leads and
 *  those above them mark each team's first responders. */
const SRE_CHAINS: readonly ChainOption[] = [
  {
    code: "SRE_SAAS",
    label: "SaaS SRE",
    rota: { family: "SRE", rotaCode: "SRE_SAAS" },
  },
  {
    code: "SRE_IAAS",
    label: "IaaS SRE",
    rota: { family: "SRE", rotaCode: "SRE_IAAS" },
  },
  {
    code: "SRE_U2",
    label: "U2-SRE",
    rota: { family: "SRE", rotaCode: "SME_U2" },
    sreTeamKey: "u2",
  },
  {
    code: "SRE_B_CENTRAL",
    label: "B-Central-SRE",
    rota: { family: "SRE", rotaCode: "SME_B_CENTRAL" },
    sreTeamKey: "b-central",
  },
  { code: "PAAS", label: "PaaS SRE", disabledReason: "No PaaS rota yet" },
  { code: "SME", label: "SME", rota: { family: "SME" } },
];

interface Props {
  /** Switch the page to the week view on a rota (and team, zone, day). */
  onOpenRota: (target: PagingRotaTarget) => void;
  /** The chain the tab opens on: the reader's own. Its family (CRE, else
   *  SRE) is the tab it opens on. */
  initialChain?: PagingChainCode;
  /** The chains this reader is shown -- their own (caseAlertChainsFor).
   *  Every chain when absent, as for a CSM admin or the CRE and CS heads. */
  visibleChains?: readonly CaseAlertChain[];
  /** The Team Schedule's teams, for the SME teams' chat spaces. */
  teams?: readonly Pick<ScheduleTeam, "key" | "name" | "family">[];
  /** Teams people are only moved onto (Migration, SaaS Night cover). */
  moveTargetKeys?: readonly string[];
}

/** Whether the reader may change both ladders' settings: what a CSM admin
 *  or portal admin gets, a role not tied to any one team. */
function managesBothLadders(settings?: AlertSettingsData): boolean {
  const ladders = settings?.canManage.roomLadders ?? [];
  return ladders.includes("CRE") && ladders.includes("SRE");
}

/**
 * The Alert Settings tab: a family tab for CRE and one for SRE (SaaS, IaaS and
 * SME together), and under it, where both apply, "Who gets alerts" and
 * "Settings". CRE's Who gets alerts is its chain, edited here: each team's
 * first responders, its Team lead and the levels above. SRE's lists each SRE
 * and SME chain with its readiness and where its people are set (they come
 * from the rota). Settings holds the family's settings, kept as changes until
 * its Submit, for those who may change them. The last sub-tab is kept for
 * this browser.
 */
export default function CasePagingTab(props: Props): JSX.Element {
  // One list of changes for both families, so moving between them keeps both.
  return (
    <CaseAlertDraftsProvider>
      <AlertTabs {...props} />
    </CaseAlertDraftsProvider>
  );
}

function AlertTabs({
  onOpenRota,
  initialChain = "SRE_SAAS",
  visibleChains,
  teams,
  moveTargetKeys,
}: Props): JSX.Element {
  const alertSettings = useCaseAlertSettings();
  const roomLadders = alertSettings.data?.canManage.roomLadders;
  const manages = (code: CaseAlertChain) =>
    code !== "PAAS" && mayManageChain(code, roomLadders);
  // The SRE and SME chains the reader is shown: their own, and every one
  // whose settings they may change.
  const isSRETeam = (key: string) =>
    (teams ?? []).some((t) => t.key.toLowerCase() === key && t.family === "SRE");
  const sreChains = SRE_CHAINS.filter(
    (c) =>
      (!c.sreTeamKey || isSRETeam(c.sreTeamKey)) &&
      (!visibleChains || visibleChains.includes(c.code) || manages(c.code)),
  );
  // CRE's chain: for every chain's reader (a CSM admin, the heads) and for
  // whoever caseAlertChainsFor gave it to (a CRE Team lead, rota admin,
  // America lead).
  const creChainShown = !visibleChains || visibleChains.includes("CRE");
  const families: SettingsFamily[] = [
    ...(creChainShown || mayManageFamily("CRE", roomLadders) ? (["CRE"] as const) : []),
    ...(sreChains.length > 0 || mayManageFamily("SRE", roomLadders) ? (["SRE"] as const) : []),
  ];
  // The reader's pick, else their own family, else the first offered. A CSM
  // or portal admin belongs to no one family: they open on the first.
  const [familyChoice, setFamily] = useState<SettingsFamily | undefined>(undefined);
  const own: SettingsFamily = initialChain === "CRE" ? "CRE" : "SRE";
  const family: SettingsFamily | undefined =
    familyChoice && families.includes(familyChoice)
      ? familyChoice
      : managesBothLadders(alertSettings.data)
        ? families[0]
        : families.includes(own)
          ? own
          : families[0];
  const [view, setView] = useState<ChainView>(readView);
  const chooseView = (next: ChainView) => {
    setView(next);
    writeView(next);
  };
  const readiness = useGetPagingReadiness(PAGING_READINESS_DAYS);
  const hasSettings = family !== undefined && mayManageFamily(family, roomLadders);
  const hasChains =
    family === "SRE" ? sreChains.length > 0 : family === "CRE" && creChainShown;
  const shown: ChainView = !hasChains ? "settings" : !hasSettings ? "chain" : view;
  // The people a phone gap can be acted on for: the chain on screen, CRE's
  // or SRE's. SME's people are not served as a chain, so its phone gaps keep
  // the profile hint. CRE's is the same read as the panel's: no extra request.
  const creOnScreen = family === "CRE" && hasChains && shown === "chain";
  const sreOnScreen = family === "SRE" && hasChains && shown === "chain";
  const creChain = useGetPagingChain("CRE", creOnScreen);
  const sreChain = useGetPagingChain("SRE", sreOnScreen);
  const onScreen = creOnScreen ? creChain : sreOnScreen ? sreChain : undefined;
  const members: PagingChainMember[] = onScreen?.data?.members ?? [];
  const readAt = onScreen?.dataUpdatedAt ?? 0;
  const { controls: phone, dialog } = usePagingPhoneControls(members, readAt);

  /** "Test call" on a phone gap for a number the reader may test (a portal
   *  or rota admin). A missing number gets no action here: it is added on the
   *  person's profile, which the gap's hint says. */
  const phoneActionFor = (gap: PagingReadinessGap): PhoneGapAction | null => {
    if (!PHONE_GAPS.has(gap.code) || !gap.userId || gap.code === "NO_PHONE")
      return null;
    const m = members.find(
      (x) => x.userId === gap.userId && x.canTestCall === true,
    );
    if (!m?.testCall) return null;
    const calling = phone.calling(m);
    return {
      label: calling ? CALLING_LABEL : "Test call",
      onClick: () => phone.test(m),
      pending: calling,
    };
  };

  if (!family) {
    return <div className="offnone">There is no alert chain of yours to show.</div>;
  }

  // The family tabs; one family has nothing to pick between.
  const familyTabs =
    families.length < 2 ? null : (
      <div className="seg teamseg cp-families" role="tablist" aria-label="Family">
        {families.map((f) => (
          <button
            key={f}
            type="button"
            role="tab"
            aria-selected={family === f}
            className={family === f ? "on" : undefined}
            onClick={() => setFamily(f)}
          >
            {f}
          </button>
        ))}
      </div>
    );
  // Under the family: who gets alerts, and its settings, where both apply.
  const subTabs =
    hasChains && hasSettings ? (
      <div className="cp-subtabs" role="tablist" aria-label="Show">
        {(
          [
            ["chain", "Who gets alerts"],
            ["settings", "Settings"],
          ] as const
        ).map(([v, label]) => (
          <button
            key={v}
            type="button"
            role="tab"
            aria-selected={shown === v}
            className={shown === v ? "on" : undefined}
            onClick={() => chooseView(v)}
          >
            {label}
          </button>
        ))}
      </div>
    ) : null;

  return (
    <>
      {familyTabs || subTabs ? (
        <div className="cp-tabs">
          {familyTabs}
          {subTabs}
        </div>
      ) : null}
      {shown === "settings" ? (
        <CaseAlertSettings
          key={family}
          family={family}
          teams={teams}
          moveTargetKeys={moveTargetKeys}
        />
      ) : family === "CRE" ? (
        <CasePagingPanel
          readiness={{
            chain: readiness.data?.chains.find((x) => x.chain === "CRE"),
            days: PAGING_READINESS_DAYS,
            isLoading: readiness.isLoading,
            isError: readiness.isError,
          }}
          onOpenRota={(gap) =>
            onOpenRota({
              family: "CRE",
              teamKey: gap.teamKey,
              zoneCode: gap.zoneCode,
              date: gap.date,
            })
          }
          phone={phone}
          phoneActionFor={phoneActionFor}
        />
      ) : (
        sreChains.map((c) => {
          const chainReadiness = readiness.data?.chains.find((x) => x.chain === c.code);
          const rota = c.rota;
          /** A rota gap opens the rota the gap is on: its team's, its
           *  zone's, or the chain's own. */
          const openGap = (gap: PagingReadinessGap) =>
            onOpenRota({
              family: rota?.family ?? "SRE",
              rotaCode: rota?.rotaCode,
              teamKey: gap.teamKey,
              zoneCode: gap.zoneCode,
              date: gap.date,
            });
          return (
            <section key={c.code} className="cp-chain" aria-labelledby={`cp-h-${c.code}`}>
              <h2 id={`cp-h-${c.code}`}>{c.label} alert chain</h2>
              {c.disabledReason ? (
                <p className="cp-from">{c.label} has no rota yet, so there is no alert chain to show.</p>
              ) : (
                <>
                  <PagingReadinessStrip
                    label={c.label}
                    chain={chainReadiness}
                    days={PAGING_READINESS_DAYS}
                    isLoading={readiness.isLoading}
                    isError={readiness.isError}
                    onOpenRota={openGap}
                    phoneActionFor={phoneActionFor}
                  />
                  <p className="cp-from">
                    People come from the {c.label} rota; set them in the Team
                    Schedule views.{" "}
                    {rota ? (
                      <button
                        type="button"
                        className="btn sm"
                        onClick={() =>
                          onOpenRota({ family: rota.family, rotaCode: rota.rotaCode })
                        }
                      >
                        Open the {c.label} rota
                      </button>
                    ) : null}
                  </p>
                </>
              )}
            </section>
          );
        })
      )}
      {dialog}
    </>
  );
}
