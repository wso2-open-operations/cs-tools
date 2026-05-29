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
import { useQuery } from "@tanstack/react-query";

import { cases } from "@features/case-types/cases/api/cases.queries";
import { changeRequests } from "@features/case-types/change-requests/api/changes.queries";
import { chats } from "@features/case-types/conversations/api/chats.queries";
import { serviceRequests } from "@features/case-types/service-requests/api/service-requests.queries";
import { useRequiredParams } from "@features/detail/hooks";

import { useIssueType } from "@shared/hooks";

export function useCase() {
  const { id } = useRequiredParams();
  const query = useQuery(cases.get(id));
  const { label: issueType } = useIssueType(query.data?.issueTypeId);

  return {
    ...query,
    data: query.data ? { ...query.data, issueType } : undefined,
  };
}

export function useChat() {
  const { id } = useRequiredParams();
  const query = useQuery(chats.get(id));

  return query;
}

export function useServiceRequest() {
  const { id } = useRequiredParams();
  const query = useQuery(serviceRequests.get(id));

  return query;
}

export function useChangeRequest() {
  const { id } = useRequiredParams();
  const query = useQuery(changeRequests.get(id));

  return query;
}
