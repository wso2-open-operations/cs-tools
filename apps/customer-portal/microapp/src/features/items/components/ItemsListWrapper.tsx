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
import { Fragment, type ReactNode } from "react";

import { useFilters } from "@features/items/hooks";

import { GroupAccordion } from "@shared/components/ui/GroupAccordion";

import type { CaseType } from "@shared/types";

export function ItemsListWrapper({ type, count, children }: { type: CaseType; count?: number; children: ReactNode }) {
  const { filters } = useFilters();

  if (filters.types.length > 1)
    return (
      <GroupAccordion type={type} count={count}>
        {children}
      </GroupAccordion>
    );
  return <Fragment>{children}</Fragment>;
}
