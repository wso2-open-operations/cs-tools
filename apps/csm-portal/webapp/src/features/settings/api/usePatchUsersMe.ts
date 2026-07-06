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
import { useAuthApiClient } from "@hooks/useAuthApiClient";
import { apiConfig } from "@config/apiConfig";
import { ApiError, parseApiResponseMessage } from "@utils/ApiError";

// PATCH /users/me accepts phoneNumber (E.164, via SCIM) and timeZone (via the
// entity service); at least one is required. The response echoes only the
// field(s) that were updated. No timeZone-editing UI exists yet — the field is
// here for contract parity and a future profile editor.
export interface UpdateUserPayload {
  phoneNumber?: string;
  timeZone?: string;
}

export interface UpdatedUserResponse {
  phoneNumber?: string;
  timeZone?: string;
}

export function usePatchUsersMe() {
  const authFetch = useAuthApiClient();
  const qc = useQueryClient();

  return useMutation<UpdatedUserResponse, Error, UpdateUserPayload>({
    mutationFn: async (payload) => {
      const res = await authFetch(`${apiConfig.backendUrl}/users/me`, {
        method: "PATCH",
        body: JSON.stringify(payload),
      });
      if (!res.ok) {
        const body = await res.text();
        throw new ApiError(
          res.status,
          res.statusText,
          parseApiResponseMessage(body, res.status, res.statusText),
        );
      }
      return (await res.json()) as UpdatedUserResponse;
    },
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: ["users-me"] });
    },
  });
}
