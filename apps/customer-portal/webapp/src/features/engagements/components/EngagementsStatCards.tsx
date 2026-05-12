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

import { type JSX, useMemo } from "react";
import ListStatGrid from "@components/list-view/ListStatGrid";
import {
  ENGAGEMENTS_STAT_CARDS_CONFIG,
  ENGAGEMENTS_STAT_GRID_ENTITY_NAME,
} from "@/features/engagements/constants/engagements";
import type {
  EngagementsStatCardsProps,
  EngagementsStatKey,
} from "@features/engagements/types/engagements";
import { computeEngagementsStatValues } from "@features/engagements/utils/engagements";

/**
 * Key metrics for engagements (total, active, completed, on hold).
 *
 * @param props - Stats payload and loading/error flags.
 * @returns {JSX.Element} Stat card grid.
 */
export default function EngagementsStatCards({
  stats,
  isLoading,
  isError,
  onStatClick,
}: EngagementsStatCardsProps): JSX.Element {
  const flattened = useMemo(() => computeEngagementsStatValues(stats), [stats]);

  return (
    <ListStatGrid<EngagementsStatKey>
      isLoading={isLoading}
      configs={ENGAGEMENTS_STAT_CARDS_CONFIG}
      stats={flattened}
      isError={isError}
      entityName={ENGAGEMENTS_STAT_GRID_ENTITY_NAME}
      itemSize={{ xs: 12, sm: 4, md: 4 }}
      onStatClick={onStatClick}
    />
  );
}
