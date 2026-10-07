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

/** Words kept upper-case wherever they appear in a team's name. */
const ACRONYMS = new Set(["abt", "cre", "sre", "sme", "iaas", "fde"]);

/**
 * A team's name as a reader should see it. Team names are synced from the
 * directory, which spells them as identifiers ("Atlas_abt_cre_team",
 * "customer_onboarding_team"), and a team's key is that name in lower case.
 * The trailing "abt cre team" / "cre team" / "team" says what the page
 * already shows, so it is dropped; the rest is split into words and
 * capitalised: "Atlas", "Customer Onboarding", "CRE Leadership".
 *
 * A name that is already written for people ("CRE Leadership") comes back
 * unchanged, and one with nothing left after the suffix keeps its words.
 */
export function teamDisplayName(raw: string): string {
  const words = raw.trim().split(/[\s_-]+/).filter(Boolean);
  if (words.length === 0) return raw;
  const lower = words.map((w) => w.toLowerCase());

  let end = lower.length;
  if (end > 1 && lower[end - 1] === "team") {
    end--;
    if (end > 1 && (lower[end - 1] === "cre" || lower[end - 1] === "sre")) {
      end--;
      if (end > 1 && lower[end - 1] === "abt") end--;
    }
  }

  return words
    .slice(0, end)
    .map((w, i) => {
      const l = lower[i];
      if (ACRONYMS.has(l)) return l.toUpperCase();
      // Mixed or upper case is how somebody chose to write it ("E2ECABAP").
      if (w !== l) return w;
      return l.charAt(0).toUpperCase() + l.slice(1);
    })
    .join(" ");
}
