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

import { createContext, useContext } from "react";
import { teamColour } from "./rotaHues";
import { teamDisplayName } from "./teamDisplayName";

/**
 * A team's colour, by its position in the list the catalogue serves.
 *
 * A context rather than a prop because the colour is wanted deep inside these
 * cards -- a name row, an off-rota stack, a team split -- and threading a
 * lookup through every one of them would be a lot of plumbing for a value
 * that does not change while the page is open.
 *
 * Defaults to a neutral for every team, so a card rendered outside the
 * provider draws in grey rather than throwing.
 */
export const TeamColourContext = createContext<(teamKey: string) => string>(() =>
  teamColour(undefined),
);

/** The colour for a team, from whatever list the page was given. */
export function useTeamColour(): (teamKey: string) => string {
  return useContext(TeamColourContext);
}

/**
 * A team's display name, by its key.
 *
 * The rota stores team_key, which is a slug -- since the catalogue moved to
 * the long convention it reads "castor_abt_cre_team", which is a database
 * identifier and not a thing to show a reader. The catalogue serves the name
 * beside the key, so the lookup rides along with the colour rather than being
 * threaded separately through every card.
 *
 * Falls back to the key itself, tidied by teamDisplayName, so a team the
 * catalogue has not got still renders something recognisable instead of an
 * empty label.
 */
export const TeamNameContext = createContext<(teamKey: string) => string>(teamDisplayName);

/** The display name for a team, from whatever list the page was given. */
export function useTeamName(): (teamKey: string) => string {
  return useContext(TeamNameContext);
}
