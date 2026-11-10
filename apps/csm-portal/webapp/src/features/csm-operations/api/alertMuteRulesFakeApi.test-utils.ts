// Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
//
// WSO2 LLC. licenses this file to you under the Apache License,
// Version 2.0 (the "License"); you may not use this file except
// in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.

import type {
  AlertMuteRuleInput,
  AlertMuteRule,
  AlertMuteRuleSearchPayload,
  AlertMuteRuleSearchResponse,
} from "@features/csm-operations/types/alertMuteRules";
import { alertMuteRuleState, matchesStateFilter } from "@features/csm-operations/utils/alertMuteRules";

/**
 * Tests only: an in-memory `/alert-mute-rules` backend shaped like
 * `useBackendApi()`, with entity-service's state rules (edit and delete while
 * scheduled, end while active), so the pages can be driven end to end.
 */
export function createFakeAlertMuteRulesApi(actor = "engineer@example.test") {
  let rules: AlertMuteRule[] = [];
  let seq = 0;

  const find = (path: string): AlertMuteRule => {
    const id = decodeURIComponent(path.split("/")[2]);
    const rule = rules.find((r) => r.id === id);
    if (!rule) throw new Error("alert mute rule not found");
    return rule;
  };
  const save = (rule: AlertMuteRule): AlertMuteRule => {
    rules = rules.map((r) => (r.id === rule.id ? rule : r));
    return rule;
  };
  const stamp = (): string => new Date().toISOString();

  const api = {
    get: async <T,>(path: string): Promise<T | null> => find(path) as T,
    post: async <TReq, TRes>(path: string, body: TReq): Promise<TRes> => {
      if (path === "/alert-mute-rules/search") {
        const { filters, pagination } = body as AlertMuteRuleSearchPayload;
        const q = filters.searchQuery?.toLowerCase();
        const hits = [...rules]
          .reverse()
          .filter((r) => !filters.states || filters.states.some((s) => matchesStateFilter(alertMuteRuleState(r, Date.now()), s)))
          .filter((r) => !q || [r.name, r.number, r.description, ...r.conditions.map((c) => c.value)].some((v) => v.toLowerCase().includes(q)));
        const res: AlertMuteRuleSearchResponse = {
          alertMuteRules: hits.slice(pagination.offset, pagination.offset + pagination.limit),
          total: hits.length,
          offset: pagination.offset,
          limit: pagination.limit,
        };
        return res as TRes;
      }
      if (path === "/alert-mute-rules") {
        seq += 1;
        const now = stamp();
        const rule: AlertMuteRule = {
          ...(body as AlertMuteRuleInput),
          id: `ab-${seq}`,
          number: `ABR${String(seq).padStart(7, "0")}`,
          createdBy: actor,
          createdOn: now,
          updatedBy: actor,
          updatedOn: now,
          endedBy: null,
          endedOn: null,
        };
        rules = [...rules, rule];
        return rule as TRes;
      }
      if (path.endsWith("/extend")) {
        return save({ ...find(path), endOn: (body as { endOn: string }).endOn, updatedOn: stamp() }) as TRes;
      }
      throw new Error(`unexpected POST ${path}`);
    },
    postEmpty: async <TRes,>(path: string): Promise<TRes> => {
      const rule = find(path);
      if (alertMuteRuleState(rule, Date.now()) !== "active") throw new Error("only an active alert mute rule can be ended");
      return save({ ...rule, endedBy: actor, endedOn: stamp() }) as TRes;
    },
    patch: async <TReq, TRes>(path: string, body: TReq): Promise<TRes> => {
      const rule = find(path);
      if (alertMuteRuleState(rule, Date.now()) !== "scheduled") throw new Error("only a scheduled alert mute rule can be edited");
      return save({ ...rule, ...(body as AlertMuteRuleInput), updatedBy: actor, updatedOn: stamp() }) as TRes;
    },
    delete: async (path: string): Promise<void> => {
      const rule = find(path);
      if (alertMuteRuleState(rule, Date.now()) !== "scheduled") throw new Error("only a scheduled alert mute rule can be deleted");
      rules = rules.filter((r) => r.id !== rule.id);
    },
  };

  return {
    api,
    /** Adds a rule directly, as if created earlier. */
    seed: (input: AlertMuteRuleInput, extra: Partial<AlertMuteRule> = {}): AlertMuteRule => {
      seq += 1;
      const rule: AlertMuteRule = {
        ...input,
        id: `ab-${seq}`,
        number: `ABR${String(seq).padStart(7, "0")}`,
        createdBy: "a@example.test",
        createdOn: stamp(),
        updatedBy: "a@example.test",
        updatedOn: stamp(),
        endedBy: null,
        endedOn: null,
        ...extra,
      };
      rules = [...rules, rule];
      return rule;
    },
    all: (): AlertMuteRule[] => rules,
  };
}
