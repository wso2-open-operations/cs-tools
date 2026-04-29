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

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useNavigate } from "react-router-dom";
import { cases } from "@features/cases/api/cases.queries";
import { useFilters } from "@context/filters";
import { useNotify } from "@context/snackbar";
import { useDetailComments } from "@shared/hooks/useDetailComments";
import { useAttachmentPreview } from "@features/cases/hooks/useAttachmentPreview";
import { getCaseMenuOptions } from "@features/cases/services/caseActions.service";

export function useCaseDetail(id: string) {
  const navigate = useNavigate();
  const notify = useNotify();
  const queryClient = useQueryClient();

  const { data, isLoading } = useQuery(cases.get(id));
  const { data: attachments, isLoading: isAttachmentsLoading } = useQuery({
    ...cases.attachments(id),
    enabled: Boolean(id),
  });
  const { data: filters, isLoading: isFiltersLoading } = useFilters();

  const editCaseMutation = useMutation({
    ...cases.edit(id),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: cases.get(id).queryKey }),
    onError: () => notify.error("Failed to update case. Please try again."),
  });

  const menuOptions = data
    ? getCaseMenuOptions(data.statusId, {
        onResolve: () => editCaseMutation.mutate({ stateKey: 3 }),
        onMarkWaiting: () => editCaseMutation.mutate({ stateKey: 1003 }),
        onCreateRelated: () => navigate("/create", { state: { case: data } }),
      })
    : [];

  const comments = useDetailComments(id);
  const attachmentPreview = useAttachmentPreview();

  const issueType = filters?.issueTypes.find((issueType) => issueType.id === data?.issueTypeId)?.label;

  return {
    data,
    isLoading,
    attachments,
    isAttachmentsLoading,
    isFiltersLoading,
    issueType,
    menuOptions,
    comments,
    attachmentPreview,
  };
}
