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

import { useMemo, type JSX, type ReactNode } from "react";
import type { ScheduleTeam } from "../types";
import { teamColour } from "./rotaHues";
import { teamDisplayName } from "./teamDisplayName";
import { TeamColourContext, TeamNameContext } from "./teamColourContext";

/** Gives everything below it the colour for a team. See TeamColourContext. */
export function TeamColourProvider({
  teams,
  children,
}: {
  teams: readonly ScheduleTeam[];
  children: ReactNode;
}): JSX.Element {
  const colourOf = useMemo(() => {
    // A team's place within its own group -- CRE, SRE or SME -- in the
    // catalogue's order. One group is on screen at a time, so counting within
    // it keeps every team there on a palette colour of its own; counted across
    // every group, the SME rotas pushed the list past the palette and teams
    // began sharing colours.
    const order = new Map<string, number>();
    const seen = new Map<string, number>();
    for (const t of [...teams].sort((a, b) => a.sortOrder - b.sortOrder)) {
      const i = seen.get(t.family) ?? 0;
      order.set(t.key.toLowerCase(), i);
      seen.set(t.family, i + 1);
    }
    return (teamKey: string): string => teamColour(order.get(teamKey.toLowerCase()));
  }, [teams]);

    const nameOf = useMemo(() => {
    const byKey = new Map(teams.map((t) => [t.key.toLowerCase(), teamDisplayName(t.name)]));
    return (teamKey: string): string => byKey.get(teamKey.toLowerCase()) ?? teamDisplayName(teamKey);
  }, [teams]);

  return (
    <TeamColourContext.Provider value={colourOf}>
      <TeamNameContext.Provider value={nameOf}>{children}</TeamNameContext.Provider>
    </TeamColourContext.Provider>
  );
}
