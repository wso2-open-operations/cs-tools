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

import { Stack } from "@wso2/oxygen-ui";
import { type JSX } from "react";
import type { CallRequest } from "@features/support/types/calls";
import CallRequestCard from "@case-details-calls/CallRequestCard";

export interface CallRequestListProps {
  requests: CallRequest[];
  userTimeZone?: string;
  onEditClick?: (call: CallRequest) => void;
  onDeleteClick?: (call: CallRequest) => void;
  onApproveClick?: (call: CallRequest) => void;
  onRejectClick?: (call: CallRequest) => void;
}

/**
 * Renders a list of call request cards.
 *
 * @param {CallRequestListProps} props - The list of call requests and edit handler.
 * @returns {JSX.Element} The rendered list.
 */
export default function CallRequestList({
  requests,
  userTimeZone,
  onEditClick,
  onDeleteClick,
  onApproveClick,
  onRejectClick,
}: CallRequestListProps): JSX.Element {
  return (
    <Stack spacing={2}>
      {requests.map((call) => (
        <CallRequestCard
          key={call.id}
          call={call}
          userTimeZone={userTimeZone}
          onEditClick={onEditClick}
          onDeleteClick={onDeleteClick}
          onApproveClick={onApproveClick}
          onRejectClick={onRejectClick}
        />
      ))}
    </Stack>
  );
}
