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
// software distributed under the License is distributed on
// an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.

const NOVERA_CHAT_ENABLED_KEY = "novera_chat_enabled";
const SIDEBAR_COLLAPSED_KEY = "sidebar_collapsed";

/**
 * Reads whether Novera chat assistant is enabled from localStorage.
 * Defaults to true when not set.
 *
 * @returns {boolean} Whether Novera chat is enabled.
 */
export function getNoveraChatEnabled(): boolean {
  try {
    const stored = localStorage.getItem(NOVERA_CHAT_ENABLED_KEY);
    if (stored === null) return true;
    return stored === "true";
  } catch {
    return true;
  }
}

/**
 * Persists the Novera chat enabled state.
 *
 * @param {boolean} enabled - Whether Novera chat should be enabled.
 */
export function setNoveraChatEnabled(enabled: boolean): void {
  try {
    localStorage.setItem(NOVERA_CHAT_ENABLED_KEY, String(enabled));
  } catch {}
}

/**
 * Reads the sidebar collapsed state from localStorage.
 * Defaults to false (expanded) when not set.
 *
 * @returns {boolean} Whether the sidebar is collapsed.
 */
export function getSidebarCollapsed(): boolean {
  try {
    const stored = localStorage.getItem(SIDEBAR_COLLAPSED_KEY);
    if (stored === null) return false;
    return stored === "true";
  } catch {
    return false;
  }
}

/**
 * Persists the sidebar collapsed state.
 *
 * @param {boolean} collapsed - Whether the sidebar is collapsed.
 */
export function setSidebarCollapsed(collapsed: boolean): void {
  try {
    localStorage.setItem(SIDEBAR_COLLAPSED_KEY, String(collapsed));
  } catch {}
}
