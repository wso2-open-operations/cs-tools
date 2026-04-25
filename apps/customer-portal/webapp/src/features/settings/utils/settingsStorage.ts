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
const PENDING_SUCCESS_MESSAGE_KEY = "pending_success_message";
const PENDING_SETTINGS_TAB_KEY = "pending_settings_tab";
const PENDING_CASE_DETAILS_TAB_KEY = "pending_case_details_tab";


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
 * Only stores the id when it is exactly 32 characters (valid project id format).
 *
 * @param {string} projectId - The selected project id.
 */
export function setLastSelectedProjectId(projectId: string): void {
  if (projectId.length !== 32) return;
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

/**
 * Stores the settings tab to restore after the next page reload.
 *
 * @param {string} tabId - The tab id to restore.
 */
export function setPendingSettingsTab(tabId: string): void {
  try {
    sessionStorage.setItem(PENDING_SETTINGS_TAB_KEY, tabId);
  } catch {
    return;
  }
}

/**
 * Reads and clears the pending post-reload settings tab.
 *
 * @returns {string | null} The tab id, or null if none is pending.
 */
export function consumePendingSettingsTab(): string | null {
  try {
    const tab = sessionStorage.getItem(PENDING_SETTINGS_TAB_KEY);
    if (tab !== null) sessionStorage.removeItem(PENDING_SETTINGS_TAB_KEY);
    return tab;
  } catch {
    return null;
  }
}

/**
 * Stores a success message to be shown after the next page reload.
 *
 * @param {string} message - The success message.
 */
export function setPendingSuccessMessage(message: string): void {
  try {
    sessionStorage.setItem(PENDING_SUCCESS_MESSAGE_KEY, message);
  } catch {
    return;
  }
}

/**
 * Reads and clears the pending post-reload success message.
 *
 * @returns {string | null} The message, or null if none is pending.
 */
export function consumePendingSuccessMessage(): string | null {
  try {
    const msg = sessionStorage.getItem(PENDING_SUCCESS_MESSAGE_KEY);
    if (msg !== null) sessionStorage.removeItem(PENDING_SUCCESS_MESSAGE_KEY);
    return msg;
  } catch {
    return null;
  }
}

/**
 * Stores the case details tab index to restore after the next page reload.
 *
 * @param {string} tabIndex - The visible tab index to restore.
 */
export function setPendingCaseDetailsTab(tabIndex: string): void {
  try {
    sessionStorage.setItem(PENDING_CASE_DETAILS_TAB_KEY, tabIndex);
  } catch {
    return;
  }
}

/**
 * Reads and clears the pending post-reload case details tab index.
 *
 * @returns {string | null} The tab index string, or null if none is pending.
 */
export function consumePendingCaseDetailsTab(): string | null {
  try {
    const tab = sessionStorage.getItem(PENDING_CASE_DETAILS_TAB_KEY);
    if (tab !== null) sessionStorage.removeItem(PENDING_CASE_DETAILS_TAB_KEY);
    return tab;
  } catch {
    return null;
  }
}
