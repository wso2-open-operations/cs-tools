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

import { createSavedFilterViewsStore } from "@features/csm-operations/utils/savedFilterViews";

/** Saved views for the Incidents tab, on their own `localStorage` key —
 * independent of the Cases list's and the other Operations sub-tabs' own
 * saved views. */
export const incidentsSavedViews = createSavedFilterViewsStore(
  "csm.savedFilters.incidents.v1",
);
