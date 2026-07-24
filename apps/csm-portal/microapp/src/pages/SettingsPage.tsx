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

import Settings from "@components/settings/Settings";

// Reached from More > Settings, mirroring the webapp's /admin route. Settings itself is
// self-contained (components/settings/Settings.tsx) — this page is just its mount point, so it
// stays trivial to move elsewhere again later.
export default function SettingsPage() {
  return <Settings />;
}
