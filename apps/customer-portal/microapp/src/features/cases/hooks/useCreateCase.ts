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
import { useLocation } from "react-router-dom";

import { useMutation } from "@tanstack/react-query";

import { useNotify } from "@context/snackbar";

import { cases } from "@features/cases/api/cases.queries";
import type { Case, CaseClassificationResponseDto } from "@features/cases/types";
import type { ChatMessage } from "@features/chats/types";

interface CreateCaseNavigationState {
  messages?: ChatMessage[];
  classifications?: CaseClassificationResponseDto;
  case?: Case;
}

export function useCreateCase() {
  const notify = useNotify();
  const location = useLocation();
  const state = location.state as CreateCaseNavigationState | null;

  const mutation = useMutation({
    ...cases.create,
    onError: () => {
      notify.error("Failed to create case. Please try again.");
    },
  });

  return {
    state: {
      messages: state?.messages || [],
      classifications: state?.classifications ?? null,
      case: state?.case ?? null,
    },
    create: mutation,
  };
}
