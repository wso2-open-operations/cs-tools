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

import { parseApiDate } from "@shared/utils/date.utils";
import type { AttachmentDto, CaseDto, CasesDto, CommentDto } from "@features/cases/types/case.dto";
import type { Attachment, Case, CaseSummary, Comment } from "@features/cases/types/case.model";

export function toCaseSummary(dto: CasesDto["cases"][number]): CaseSummary {
  return {
    id: dto.id,
    internalId: dto.internalId,
    number: dto.number,
    createdOn: parseApiDate(dto.createdOn),
    createdBy: dto.createdBy,
    title: dto.title,
    description: dto.description ?? "",
    assigned: dto.assignedEngineer?.label,
    statusId: dto.status.id,
    severityId: dto.severity?.id,
    issueTypeId: dto.issueType?.id,
    deployment: dto.deployment?.label,
    engagementType: dto.engagementType?.label,
  };
}

export function toCase(dto: CaseDto): Case {
  return {
    id: dto.id,
    internalId: dto.internalId,
    number: dto.number,
    createdOn: parseApiDate(dto.createdOn),
    createdBy: dto.createdBy,
    updatedOn: parseApiDate(dto.updatedOn),
    title: dto.title,
    description: dto.description ?? "",
    assigned: dto.assignedEngineer?.label,
    statusId: dto.status.id,
    severityId: dto.severity?.id,
    issueTypeId: dto.issueType?.id,
    product: dto.deployedProduct ? `${dto.deployedProduct.label} ${dto.deployedProduct.version}` : undefined,
    deployment: dto.deployment?.label ?? undefined,
    reporter: dto.createdBy,
    account: dto.account?.label,
    parentCaseId: dto.parentCase?.id,
    conversationId: dto.conversation?.id,
    slaResponseTime: dto.slaResponseTime,
  };
}

export function toComment(dto: CommentDto): Comment {
  return {
    id: dto.id,
    content: dto.content,
    createdOn: parseApiDate(dto.createdOn),
    createdBy: dto.createdBy,
    attachments: dto.inlineAttachments.map((a) => ({
      id: a.id,
      type: "others" as const,
      fileName: a.fileName,
      downloadUrl: a.downloadUrl,
      createdOn: parseApiDate(a.createdOn),
      createdBy: a.createdBy,
    })),
  };
}

export function toAttachment(dto: AttachmentDto): Attachment {
  return {
    id: dto.id,
    type: /^image\//.test(dto.type) ? "image" : "others",
    fileName: dto.name,
    downloadUrl: dto.downloadUrl,
    createdOn: parseApiDate(dto.createdOn),
    createdBy: dto.createdBy,
  };
}
