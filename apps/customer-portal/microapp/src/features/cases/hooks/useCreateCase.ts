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

import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useNavigate } from "react-router-dom";
import { cases } from "@features/cases/api/cases.queries";
import { useNotify } from "@context/snackbar";

export function useCreateCase() {
  const navigate = useNavigate();
  const notify = useNotify();
  const queryClient = useQueryClient();

  const mutation = useMutation({
    ...cases.create,
    onSuccess: ({ id }) => {
      queryClient.invalidateQueries({ queryKey: ["cases"] });
      setTimeout(() => {
        navigate(`/cases/${id}`);
      }, 500);
    },
    onError: () => {
      notify.error("Failed to create case. Please try again.");
    },
  });

  return { mutation };
}
