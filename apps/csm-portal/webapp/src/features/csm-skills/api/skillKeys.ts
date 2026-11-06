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

/** React Query key roots for the skills feature. Every key starts with `ROOT`, so one invalidation refreshes the lot. */
const ROOT = "csm-skills" as const;

export const skillKeys = {
  root: [ROOT] as const,
  levels: [ROOT, "levels"] as const,
  categories: [ROOT, "categories"] as const,
  catalogue: [ROOT, "catalogue"] as const,
  search: (request: unknown) => [ROOT, "catalogue", "search", request] as const,
  mine: [ROOT, "mine"] as const,
  history: (skillId: string, limit: number, offset: number) =>
    [ROOT, "history", skillId, limit, offset] as const,
  managerProbe: [ROOT, "manager-probe"] as const,
  people: (request: unknown) => [ROOT, "people", request] as const,
};
