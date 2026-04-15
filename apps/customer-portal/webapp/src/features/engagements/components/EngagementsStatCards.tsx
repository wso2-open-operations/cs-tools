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
import {
  Briefcase,
  Clock,
  CircleCheck,
  CircleAlert,
} from "@wso2/oxygen-ui-icons-react";
import type { ProjectCasesStats } from "@features/support/types/cases";
import ListStatGrid from "@components/list-view/ListStatGrid";
import type { SupportStatConfig } from "@features/support/constants/supportConstants";
import {
  SUPPORT_STATE_AWAITING_INFO,
  SUPPORT_STATE_CLOSED,
  SUPPORT_STATE_WAITING_ON_WSO2,
} from "@features/support/constants/supportConstants";

type EngagementsStatKey = "total" | "active" | "completed" | "onHold";

export interface EngagementsStatCardsProps {
  stats?: ProjectCasesStats;
  isLoading: boolean;
  isError: boolean;
}

/**
 * EngagementsStatCards displays key metrics for engagements (total, active, completed, on hold).
 *
 * @param {EngagementsStatCardsProps} props - Component props.
 * @returns {JSX.Element} The rendered stat cards.
 */
export default function EngagementsStatCards({
  stats,
  isLoading,
  isError,
}: EngagementsStatCardsProps): JSX.Element {
  const configs: SupportStatConfig<EngagementsStatKey>[] = useMemo(() => {
    return [
      {
        icon: Briefcase,
        iconColor: "primary",
        key: "total",
        label: "Total Engagements",
      },
      {
        icon: Clock,
        iconColor: "info",
        key: "active",
        label: "Active Engagements",
      },
      {
        icon: CircleCheck,
        iconColor: "success",
        key: "completed",
        label: "Completed",
      },
      {
        icon: CircleAlert,
        iconColor: "warning",
        key: "onHold",
        label: "On Hold",
      },
    ];
  }, []);

  const flattened = useMemo((): Partial<Record<EngagementsStatKey, number>> => {
    const completed =
      stats?.stateCount?.find((s) => s.label === SUPPORT_STATE_CLOSED)?.count ??
      0;
    const onHold =
      (stats?.stateCount?.find((s) => s.label === SUPPORT_STATE_AWAITING_INFO)
        ?.count ?? 0) +
      (stats?.stateCount?.find((s) => s.label === SUPPORT_STATE_WAITING_ON_WSO2)
        ?.count ?? 0);
    return {
      total: stats?.totalCount ?? 0,
      active: stats?.activeCount ?? 0,
      completed,
      onHold,
    };
  }, [stats]);

  return (
    <ListStatGrid<EngagementsStatKey>
      isLoading={isLoading}
      configs={configs}
      stats={flattened}
      isError={isError}
      entityName="engagement statistics"
      itemSize={{ xs: 12, sm: 6, md: 3 }}
    />
  );
}
