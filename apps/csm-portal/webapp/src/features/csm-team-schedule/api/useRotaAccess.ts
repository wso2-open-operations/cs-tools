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
  useMutation,
  useQuery,
  useQueryClient,
  type UseMutationResult,
  type UseQueryResult,
} from "@tanstack/react-query";
import { useBackendApi } from "@api/backend/client";
import type { RotaAccess, RotaFamily } from "../types";

const ROTA_ACCESS_KEY = ["team-schedule", "rota-access"] as const;

/** Somebody who may change nothing: no tab. */
export const NO_ROTA_ACCESS: RotaAccess = {
  canManage: { rotaAdmins: false, markerFamilies: [], markerTeams: [], memberFamilies: [] },
  rotaAdmins: [],
  markers: [],
};

/** Whether the reader may change anything on the Rota access tab. */
export function canManageRotaAccess(access: RotaAccess | undefined): boolean {
  return Boolean(
    access &&
      (access.canManage.rotaAdmins ||
        access.canManage.markerFamilies.length > 0 ||
        (access.canManage.markerTeams ?? []).length > 0),
  );
}

/** Whether the reader may change this family's rota from This week: a rota
 *  admin of the family (isRotaAdminOf), or a CSM or portal admin -- who may
 *  manage the rota admins themselves (canManage.rotaAdmins) and edit every
 *  family's roster. A Team lead and a rota marker keep the roster. The server
 *  checks every write again. */
export function mayEditWeekOf(
  access: RotaAccess | undefined,
  email: string,
  family: RotaFamily,
): boolean {
  return Boolean(access?.canManage.rotaAdmins) || isRotaAdminOf(access, email, family);
}

/** Whether the reader holds an active rota admin grant for this family. A CSM
 *  or portal admin, a Team lead and a rota marker are not rota admins, so it is
 *  false for them. */
export function isRotaAdminOf(
  access: RotaAccess | undefined,
  email: string,
  family: RotaFamily,
): boolean {
  const me = email.trim().toLowerCase();
  if (!me || !access) return false;
  return access.rotaAdmins.some(
    (g) => g.active && g.family === family && g.email.trim().toLowerCase() === me,
  );
}

/**
 * Who holds a rota admin role or marks a team's rota, and what the reader may
 * change. The page asks it to decide whether to show the tab: the server
 * answers an empty tab, not a refusal, for somebody who may change nothing.
 */
export function useRotaAccess(): UseQueryResult<RotaAccess, Error> {
  const api = useBackendApi();
  return useQuery<RotaAccess, Error>({
    queryKey: ROTA_ACCESS_KEY,
    queryFn: async () => (await api.get<RotaAccess>("/team-schedule/rota-access")) ?? NO_ROTA_ACCESS,
    staleTime: 5 * 60 * 1000,
  });
}

/** One grant or removal: a family's rota admin, a team's rota marker, or a
 *  team's member (grant = add to the team, otherwise remove from it). */
export type RotaAccessChange =
  | { kind: "admin"; family: RotaFamily; userId: string; grant: boolean }
  | { kind: "marker"; teamKey: string; userId: string; grant: boolean }
  | { kind: "member"; teamKey: string; userId: string; grant: boolean };

function changePath(c: RotaAccessChange): string {
  const user = encodeURIComponent(c.userId);
  if (c.kind === "member") return `/team-schedule/teams/${encodeURIComponent(c.teamKey)}/members/${user}`;
  const scope = c.kind === "admin" ? `admins/${c.family}` : `markers/${encodeURIComponent(c.teamKey)}`;
  return `/team-schedule/rota-access/${scope}/${user}`;
}

/** The families whose teams' members the reader may add and remove. An older
 *  server does not send memberFamilies, and it has no member routes either,
 *  so nothing is offered. */
export function memberFamiliesOf(access: RotaAccess | undefined): RotaFamily[] {
  return access?.canManage.memberFamilies ?? [];
}

/**
 * Grant or remove rota access, or change a team's members. Afterwards the tab
 * is read again, and so is which teams the reader may edit -- a reader who
 * changed their own access should see the roster agree. A member change also
 * reads the catalogue again: the teams' member lists come from it.
 */
export function useChangeRotaAccess(): UseMutationResult<unknown, Error, RotaAccessChange> {
  const api = useBackendApi();
  const qc = useQueryClient();
  return useMutation<unknown, Error, RotaAccessChange>({
    mutationFn: (c) => (c.grant ? api.put<Record<string, never>, unknown>(changePath(c), {}) : api.del(changePath(c))),
    onSettled: (_data, _error, c) => {
      void qc.invalidateQueries({ queryKey: ROTA_ACCESS_KEY });
      void qc.invalidateQueries({ queryKey: ["team-schedule", "my-lead-teams"] });
      if (c.kind === "member") void qc.invalidateQueries({ queryKey: ["team-schedule", "catalogue"] });
    },
  });
}
