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

import type { CaseAlertRoomLadder, PagingChainCode } from "../types";
import type { SettingsFamily } from "./caseAlertDrafts";

export interface ChainSettings {
  /** The chain as the picker names it. */
  label: string;
  ladder: CaseAlertRoomLadder;
  /** Redirects: which team a CSM Portal team's cases or incidents go to.
   *  CRE and SRE; SME has none. */
  routes: boolean;
  /** The ladder's settings: the rest of its section of the paging file. */
  values: boolean;
}

/** The chains with settings here: each family's chat spaces, redirects and
 *  ladder settings, seen and changed only by that family's rota admins (and,
 *  for CRE, the CRE and CS heads) and by CSM admins. Both SRE chains show the
 *  one SRE ladder. What decides whether paging runs and what it may cost
 *  stays in the alert service's file. */
export const CHAIN_SETTINGS: Partial<Record<PagingChainCode, ChainSettings>> = {
  CRE: { label: "CRE", ladder: "CRE", routes: true, values: true },
  SRE_SAAS: { label: "SaaS SRE", ladder: "SRE", routes: true, values: true },
  SRE_IAAS: { label: "IaaS SRE", ladder: "SRE", routes: true, values: true },
  SRE_U2: { label: "U2-SRE", ladder: "SRE", routes: true, values: true },
  SRE_B_CENTRAL: { label: "B-Central-SRE", ladder: "SRE", routes: true, values: true },
  SME: { label: "SME", ladder: "SME", routes: false, values: true },
};

/** Whether a chain has settings on the tab (its Settings switch). */
export function chainHasSettings(chain: PagingChainCode): boolean {
  return CHAIN_SETTINGS[chain] !== undefined;
}

/** Whether the reader may see and change a chain's settings: the server's
 *  answer (canManage.roomLadders), by the chain's ladder. */
export function mayManageChain(chain: PagingChainCode, roomLadders: readonly string[] | undefined): boolean {
  const cfg = CHAIN_SETTINGS[chain];
  return !!cfg && (roomLadders ?? []).includes(cfg.ladder);
}

/** The ladders each family's tab holds: CRE's, and SRE's with SME's (the SRE
 *  tab lists SaaS, IaaS and SME teams together). */
export const FAMILY_LADDERS: Record<SettingsFamily, CaseAlertRoomLadder[]> = {
  CRE: ["CRE"],
  SRE: ["SRE", "SME"],
};

/** The family a ladder's settings are on. */
export function familyOfLadder(ladder: CaseAlertRoomLadder): SettingsFamily {
  return ladder === "CRE" ? "CRE" : "SRE";
}

/** The family a chain's settings are on. */
export function familyOfChain(chain: PagingChainCode): SettingsFamily {
  return chain === "CRE" ? "CRE" : "SRE";
}

/** Whether the reader may see and change any of a family's settings. */
export function mayManageFamily(family: SettingsFamily, roomLadders: readonly string[] | undefined): boolean {
  return FAMILY_LADDERS[family].some((l) => (roomLadders ?? []).includes(l));
}
