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

const SIDEBAR_COLLAPSED_KEY = "sidebar_collapsed";
const LAST_SELECTED_PROJECT_ID_KEY = "last_selected_project_id";
const NOVERA_CHAT_ENABLED_KEY = "novera_chat_enabled";


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
  } catch {
    return;
  }
}

/**
 * Reads the last selected project id from localStorage.
 *
 * @returns {string | null} The persisted project id, or null.
 */
export function getLastSelectedProjectId(): string | null {
  try {
    return localStorage.getItem(LAST_SELECTED_PROJECT_ID_KEY);
  } catch {
    return null;
  }
}

/**
 * Persists the last selected project id to localStorage.
 *
 * @param {string} projectId - The selected project id.
 */
export function setLastSelectedProjectId(projectId: string): void {
  try {
    localStorage.setItem(LAST_SELECTED_PROJECT_ID_KEY, projectId);
  } catch {
    return;
  }
}

/**
 * Reads whether Novera chat is enabled from localStorage.
 *
 * @returns {boolean} True when enabled, false otherwise.
 */
export function getNoveraChatEnabled(): boolean {
  try {
    return localStorage.getItem(NOVERA_CHAT_ENABLED_KEY) === "true";
  } catch {
    return false;
  }
}

/**
 * Persists whether Novera chat is enabled to localStorage.
 *
 * @param {boolean} enabled - Whether Novera chat is enabled.
 */
export function setNoveraChatEnabled(enabled: boolean): void {
  try {
    localStorage.setItem(NOVERA_CHAT_ENABLED_KEY, String(enabled));
  } catch {
    return;
  }
}
