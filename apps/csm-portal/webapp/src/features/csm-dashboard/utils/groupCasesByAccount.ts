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

export interface AccountCaseGroup {
  /** The account's id, or `""` for cases that carry no account. */
  accountId: string;
  /** The account's name, its id when the name is missing, or `NO_ACCOUNT_LABEL` when neither exists. */
  accountName: string;
  /** The account's cases, in their incoming order. */
  items: Record<string, unknown>[];
}

export const NO_ACCOUNT_LABEL = "No account";

function readAccount(item: Record<string, unknown>): { id: string; name: string } {
  const account = item.account;
  if (account && typeof account === "object") {
    const { id, name } = account as { id?: unknown; name?: unknown };
    return {
      id: typeof id === "string" ? id : "",
      name: typeof name === "string" ? name.trim() : "",
    };
  }
  return { id: "", name: "" };
}

/**
 * Groups case-search rows by account, most cases first (ties broken by
 * account name, so the order is stable between refreshes). Keyed on the
 * account id; a row with no id falls back to its name, and a row with neither
 * lands in one shared {@link NO_ACCOUNT_LABEL} group rather than vanishing, so
 * the group counts always add up to the number of rows given.
 */
export function groupCasesByAccount(items: Record<string, unknown>[]): AccountCaseGroup[] {
  const groups = new Map<string, AccountCaseGroup>();
  for (const item of items) {
    const { id, name } = readAccount(item);
    const key = id || `name:${name}`;
    let group = groups.get(key);
    if (!group) {
      group = { accountId: id, accountName: name || id || NO_ACCOUNT_LABEL, items: [] };
      groups.set(key, group);
    }
    group.items.push(item);
  }
  return [...groups.values()].sort(
    (a, b) => b.items.length - a.items.length || a.accountName.localeCompare(b.accountName),
  );
}
