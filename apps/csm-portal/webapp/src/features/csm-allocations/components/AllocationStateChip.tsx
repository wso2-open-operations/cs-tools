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

import type { JSX } from "react";
import SemanticChip from "@components/SemanticChip";
import { allocationStateLabel, allocationStateRole } from "../utils/allocations";

/** An allocation's state as a status pill with a friendly label. */
export default function AllocationStateChip({ state }: { state?: string | null }): JSX.Element {
  if (!state) return <span aria-label="No state">—</span>;
  return <SemanticChip role={allocationStateRole(state)} label={allocationStateLabel(state)} />;
}
