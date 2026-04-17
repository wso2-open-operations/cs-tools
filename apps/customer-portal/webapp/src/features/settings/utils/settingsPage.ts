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

import {
  AiAssistantPatchSuccessKind,
  RegistryTokenSubTabId,
  SettingsPageTabId,
} from "@features/settings/types/settings";
import {
  SETTINGS_AI_KB_SUCCESS_MESSAGE,
  SETTINGS_AI_NOVERA_SUCCESS_MESSAGE,
} from "@features/settings/constants/settingsConstants";

/**
 * Resolves the active Settings page tab from state (falls back to User Management).
 *
 * @param activeTab - Current tab id.
 * @returns {SettingsPageTabId} Valid tab id.
 */
export function resolveSettingsPageTabId(activeTab: string): SettingsPageTabId {
  switch (activeTab) {
    case SettingsPageTabId.USERS:
    case SettingsPageTabId.AI:
    case SettingsPageTabId.REGISTRY_TOKENS:
      return activeTab;
    default:
      return SettingsPageTabId.USERS;
  }
}

/**
 * Resolves registry token sub-tab; non-admins cannot stay on Service.
 *
 * @param tokenTab - Requested sub-tab id.
 * @param hasServiceTab - Whether Service tab is shown (admin).
 * @returns {RegistryTokenSubTabId} Valid sub-tab id.
 */
export function resolveRegistryTokenSubTabId(
  tokenTab: string,
  hasServiceTab: boolean,
): RegistryTokenSubTabId {
  switch (tokenTab) {
    case RegistryTokenSubTabId.USER:
      return RegistryTokenSubTabId.USER;
    case RegistryTokenSubTabId.SERVICE:
      return hasServiceTab
        ? RegistryTokenSubTabId.SERVICE
        : RegistryTokenSubTabId.USER;
    default:
      return RegistryTokenSubTabId.USER;
  }
}

/**
 * Toast message after a successful AI assistant settings patch.
 *
 * @param kind - Which toggle was updated.
 * @returns Success banner text.
 */
export function getAiAssistantPatchSuccessMessage(
  kind: AiAssistantPatchSuccessKind,
): string {
  switch (kind) {
    case AiAssistantPatchSuccessKind.NOVERA:
      return SETTINGS_AI_NOVERA_SUCCESS_MESSAGE;
    case AiAssistantPatchSuccessKind.KB:
      return SETTINGS_AI_KB_SUCCESS_MESSAGE;
    default:
      return SETTINGS_AI_NOVERA_SUCCESS_MESSAGE;
  }
}
