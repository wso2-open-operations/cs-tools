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

import { Grid } from "@wso2/oxygen-ui";
import {
  Briefcase,
  Clock,
  CircleCheck,
  CircleAlert,
} from "@wso2/oxygen-ui-icons-react";
import type { JSX } from "react";
import type { ProjectCasesStats } from "@models/responses";
import GenericSubCountCard from "@components/common/GenericSubCountCard";

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
  const total = stats?.totalCount ?? 0;
  const active = stats?.activeCount ?? 0;
  const completed =
    stats?.stateCount?.find((s) => s.label === "Closed")?.count ?? 0;
  const onHold =
    (stats?.stateCount?.find((s) => s.label === "Awaiting Info")?.count ?? 0) +
    (stats?.stateCount?.find((s) => s.label === "Waiting On WSO2")?.count ?? 0);

  const showPlaceholder = isLoading || isError;
  const placeholder = "—";

  return (
    <Grid container spacing={3}>
      <Grid size={{ xs: 12, md: 6, lg: 3 }}>
        <GenericSubCountCard
          label="Total Engagements"
          value={showPlaceholder ? placeholder : total}
          icon={<Briefcase size={32} />}
          color="primary.main"
        />
      </Grid>
      <Grid size={{ xs: 12, md: 6, lg: 3 }}>
        <GenericSubCountCard
          label="Active Engagements"
          value={showPlaceholder ? placeholder : active}
          icon={<Clock size={32} />}
          color="info.main"
        />
      </Grid>
      <Grid size={{ xs: 12, md: 6, lg: 3 }}>
        <GenericSubCountCard
          label="Completed"
          value={showPlaceholder ? placeholder : completed}
          icon={<CircleCheck size={32} />}
          color="success.main"
        />
      </Grid>
      <Grid size={{ xs: 12, md: 6, lg: 3 }}>
        <GenericSubCountCard
          label="On Hold"
          value={showPlaceholder ? placeholder : onHold}
          icon={<CircleAlert size={32} />}
          color="warning.main"
        />
      </Grid>
    </Grid>
  );
}

