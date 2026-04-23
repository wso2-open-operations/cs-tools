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

import { Card, CardContent } from "@wso2/oxygen-ui";
import type { JSX } from "react";
import type { UpdateProductCardProps } from "@features/updates/types/updates";
import { resolveUpdateCardHeaderStatusColor } from "@features/updates/utils/updates";
import { UpdateCardHeader } from "@update-cards/UpdateCardHeader";
import { UpdateCardLevels } from "@update-cards/UpdateCardLevels";
import { UpdateCardBreakdown } from "@update-cards/UpdateCardBreakdown";

/**
 * Card component to display update status for a specific product.
 *
 * @param {UpdateProductCardProps} props - Component props.
 * @returns {JSX.Element} The rendered component.
 */
export function UpdateProductCard({
  item,
  onInstalledClick,
  onPendingClick,
}: UpdateProductCardProps): JSX.Element {
  const {
    productName,
    productBaseVersion,
    endingUpdateLevel,
    recommendedUpdateLevel,
    installedUpdatesCount,
    installedSecurityUpdatesCount,
    availableUpdatesCount,
    availableSecurityUpdatesCount,
  } = item;

  const pendingLevels = recommendedUpdateLevel - endingUpdateLevel;
  const percentage =
    recommendedUpdateLevel > 0
      ? (endingUpdateLevel / recommendedUpdateLevel) * 100
      : 0;
  const isHealthy = percentage >= 50;
  const statusColor = resolveUpdateCardHeaderStatusColor(isHealthy);

  const installedRegular = installedUpdatesCount;
  const installedSecurity = installedSecurityUpdatesCount;
  const pendingRegular = availableUpdatesCount;
  const pendingSecurity = availableSecurityUpdatesCount;
  const totalPending = pendingRegular + pendingSecurity;

  return (
    <Card>
      <CardContent sx={{ p: 2 }}>
        <UpdateCardHeader
          productName={productName}
          productBaseVersion={productBaseVersion}
          percentage={percentage}
          statusColor={statusColor}
        />

        <UpdateCardLevels
          currentUpdateLevel={endingUpdateLevel}
          recommendedUpdateLevel={recommendedUpdateLevel}
          pendingLevels={pendingLevels}
        />

        <UpdateCardBreakdown
          installedRegular={installedRegular}
          installedSecurity={installedSecurity}
          pendingRegular={pendingRegular}
          pendingSecurity={pendingSecurity}
          totalPending={totalPending}
          onInstalledClick={onInstalledClick}
          onPendingClick={onPendingClick}
        />
      </CardContent>
    </Card>
  );
}
