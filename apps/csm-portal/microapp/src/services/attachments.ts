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

import { queryOptions } from "@tanstack/react-query";
import { ATTACHMENTS_ENDPOINT, ATTACHMENTS_SEARCH_ENDPOINT } from "@config/endpoints";
import type {
  AttachmentCreatePayloadDto,
  AttachmentCreateResponseDto,
  AttachmentDetailDto,
  AttachmentReferenceType,
  AttachmentSearchResponseDto,
} from "@src/types";
import { toCaseAttachment, type CaseAttachment } from "@src/types";
import apiClient from "./apiClient";

const createAttachment = async (payload: AttachmentCreatePayloadDto): Promise<AttachmentDetailDto> => {
  const { data } = await apiClient.post<AttachmentCreateResponseDto>(ATTACHMENTS_ENDPOINT, payload);
  return data.attachment;
};

const searchAttachments = async (
  referenceId: string,
  referenceType: AttachmentReferenceType,
): Promise<CaseAttachment[]> => {
  const { data } = await apiClient.post<AttachmentSearchResponseDto>(ATTACHMENTS_SEARCH_ENDPOINT, {
    referenceId,
    referenceType,
    pagination: { limit: 50 },
  });
  return (data.attachments ?? []).map(toCaseAttachment);
};

export const attachments = {
  create: createAttachment,
  forCase: (caseId: string) =>
    queryOptions({
      queryKey: ["case", caseId, "attachments"],
      queryFn: () => searchAttachments(caseId, "case"),
    }),
};
