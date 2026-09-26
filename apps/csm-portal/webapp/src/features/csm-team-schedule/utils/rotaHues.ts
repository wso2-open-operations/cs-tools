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

/**
 * Team colours.
 *
 * These team keys are organisation vocabulary, and committing them here is the
 * coupling CSM_TEAM_REGISTRY exists to avoid -- raised in review and correct.
 * They are kept for now regardless, deliberately: `CsmTeamSchedulePage.tsx`
 * holds the same eleven keys to drive the team picker and the absence filter,
 * so removing them from this file alone does not fix the violation, it just
 * spreads it. It also costs real colour fidelity -- deriving a hue from the key
 * collapsed eleven teams onto eight colours, because no hash can guarantee
 * separation for a key set it has never seen.
 *
 * The fix worth making is to serve the team list from the API, which the
 * registry already knows, and colour by a team's position in it: no names
 * committed anywhere, and every team keeps a distinct colour. That is its own
 * change, not a side effect of this one.
 */
export const TEAM_COLOURS: Record<string, string> = {
  americas: "#3aa889",
  castor: "#4a7fe0",
  draco: "#e8962a",
  vega: "#d95c5c",
  sirius: "#2f9e8f",
  atlas: "#8a63d2",
  phoenix: "#c9a227",
  rigel: "#c0559b",
  migration: "#6b7280",
  apollo: "#2f9e8f",
  artemis: "#8a63d2",
};

/**
 * The SRE time zones, in the prototype's own hues.
 *
 * These were JS constants there, not CSS tokens -- which is why an earlier
 * version of this page referenced --tz1-fg and friends, found nothing, and
 * drew all three lanes in the same fallback grey. A zone's colour is how a
 * reader tells the three columns apart at a glance, so it is worth being
 * explicit about.
 */
export const ZONE_COLOURS: Record<string, string> = {
  TZ1: "#e8962a",
  TZ2: "#4a7fe0",
  TZ3: "#8a63d2",
};

/** The colour for a time zone, falling back to a neutral for one not listed. */
export function zoneColour(code: string): string {
  return ZONE_COLOURS[code.toUpperCase()] ?? "#6b7280";
}

/** The colour for a team, falling back to a neutral for one not listed. */
export function teamColour(teamKey: string): string {
  return TEAM_COLOURS[teamKey.toLowerCase()] ?? "#6b7280";
}
