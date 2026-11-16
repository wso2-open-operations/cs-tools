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
import { useMemo } from "react";
import type { RotaFamily } from "../types";
import { useRotaAccess } from "./useRotaAccess";
import { useScheduleCatalogue } from "./useTeamSchedule";

/** Someone who can be copied on a reminder. */
export interface CcPerson {
  name: string;
  email: string;
}

/**
 * Everybody of a family who can be copied: the members and heads of its
 * teams, and its rota admins (who may sit on no team), each once by email,
 * by name.
 */
export function useFamilyCcPeople(family: RotaFamily): CcPerson[] {
  const catalogue = useScheduleCatalogue();
  const access = useRotaAccess();
  return useMemo(() => {
    const byEmail = new Map<string, CcPerson>();
    const add = (name: string | undefined, email: string | undefined): void => {
      const e = (email ?? "").trim();
      if (!e || byEmail.has(e.toLowerCase())) return;
      byEmail.set(e.toLowerCase(), { name: (name ?? "").trim() || e, email: e });
    };
    for (const team of catalogue.data?.teams ?? []) {
      if (team.family !== family) continue;
      for (const m of [...(team.members ?? []), ...(team.heads ?? [])]) add(m.name, m.email);
    }
    for (const a of access.data?.rotaAdmins ?? []) {
      if (a.family === family && a.active) add(a.name, a.email);
    }
    return [...byEmail.values()].sort((a, b) => a.name.localeCompare(b.name));
  }, [catalogue.data, access.data, family]);
}
