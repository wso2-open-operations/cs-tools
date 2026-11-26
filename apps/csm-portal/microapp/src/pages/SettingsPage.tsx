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
import { NoPortalAccess } from "@components/common/NoPortalAccess";
import { usePortalAccess } from "@src/access/usePortalAccess";

// Reached from More > Settings, mirroring the webapp's /admin route. The page is the
// administration user list, so it follows canViewSettings. canViewSettingsSection is the
// wider shell flag and does not open this page: a catalogue manager who is not a CS
// engineer must not start the user-list query.
export default function SettingsPage() {
  const { access } = usePortalAccess();
  if (!access.canViewSettings) return <NoPortalAccess />;
  return <Settings />;
}
