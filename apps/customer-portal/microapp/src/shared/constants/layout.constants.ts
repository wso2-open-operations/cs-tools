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
import type { LayoutDeclaration } from "@context/layout";

export const Tab = {
  None: -1,
  Home: 0,
  Support: 1,
  Users: 2,
  Profile: 3,
} as const;

export const DEFAULT_LAYOUT_CONFIG: LayoutDeclaration = {
  tabIndex: Tab.None,
  title: undefined,
  visibility: undefined,
  slots: undefined,
};
