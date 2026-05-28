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
import { CASE_TYPES } from "@shared/constants";
import type { CaseType } from "@shared/types";

export const SEARCH_PLACEHOLDER_CONFIG: Record<CaseType, string> = {
  [CASE_TYPES.DEFAULT]: "Search Cases",
  [CASE_TYPES.CHAT]: "Search Chats",
  [CASE_TYPES.SERVICE_REQUEST]: "Search Service Requests",
  [CASE_TYPES.CHANGE_REQUEST]: "Search Change Requests",
  [CASE_TYPES.SECURITY_REPORT_ANALYSIS]: "Search Security Report Analysis",
  [CASE_TYPES.ENGAGEMENT]: "Search Engagement",
  [CASE_TYPES.ANNOUNCEMENT]: "Search Announcements",
};

export const COMMENT_ENABLED_TYPES: CaseType[] = [
  CASE_TYPES.DEFAULT,
  CASE_TYPES.SERVICE_REQUEST,
  CASE_TYPES.SECURITY_REPORT_ANALYSIS,
  CASE_TYPES.ENGAGEMENT,
];
