// Copyright (c) 2026 WSO2 LLC. (https://www.wso2.com).
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

import { describe, expect, it } from "vitest";
import {
  ALERT_MUTE_RULE_ENVIRONMENT_ALIASES,
  ALERT_MUTE_RULE_FIELDS,
} from "@features/csm-operations/utils/alertMuteRules";
import coreMuteRule from "../../../../../../../integrations/sre-alert-core-service/internal/muterule/muterule.go?raw";
import entityOpenapi from "../../../../../../../entity-service/openapi.yaml?raw";

// The fields the rule form offers must be the ones entity-service accepts: the
// AlertMuteRuleCondition.field enum in entity-service/openapi.yaml, which the
// portal backend and alert-core pin their own copies to as well.
function contractFields(): string[] {
  const m = /\n {4}AlertMuteRuleCondition:\n[\s\S]*?field:\n[\s\S]*?enum: \[([^\]]*)\]/.exec(entityOpenapi);
  if (!m) throw new Error("AlertMuteRuleCondition.field enum not found in entity-service/openapi.yaml");
  return m[1]
    .split(",")
    .map((f) => f.trim())
    .sort();
}

describe("ALERT_MUTE_RULE_FIELDS", () => {
  it("matches entity-service's contract", () => {
    expect(ALERT_MUTE_RULE_FIELDS.map((f) => f.value).sort()).toEqual(contractFields());
  });
});

// The aliases the form shows under the Environment picker must be the groups
// alert-core actually matches by (environmentGroups in its muterule.go), so
// what an engineer is told is what gets muted.
function coreEnvironmentGroups(): Record<string, string[]> {
  const block = /groups := map\[string\]\[\]string\{([\s\S]*?)\n\t\}/.exec(coreMuteRule);
  if (!block) throw new Error("environmentGroups not found in alert-core's muterule.go");
  const out: Record<string, string[]> = {};
  for (const m of block[1].matchAll(/"([a-z_]+)":\s*\{([^}]*)\}/g)) {
    const names = [...m[2].matchAll(/"([^"]+)"/g)].map((n) => n[1]);
    out[m[1]] = names.filter((n) => n !== m[1]).sort();
  }
  return out;
}

describe("ALERT_MUTE_RULE_ENVIRONMENT_ALIASES", () => {
  it("matches alert-core's environment groups", () => {
    const shown = Object.fromEntries(
      Object.entries(ALERT_MUTE_RULE_ENVIRONMENT_ALIASES).map(([g, a]) => [g, [...a].sort()]),
    );
    expect(shown).toEqual(coreEnvironmentGroups());
  });
});
