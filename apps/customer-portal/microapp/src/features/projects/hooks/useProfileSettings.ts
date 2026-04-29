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

import { useMutation, useQueryClient, useSuspenseQueries } from "@tanstack/react-query";
import { useNavigate } from "react-router-dom";
import { users } from "@features/users/api/users.queries";
import { metadata } from "@features/metadata/api/metadata.queries";
import { useNotify } from "@context/snackbar";
import type { EditMeDto } from "@features/users/types/user.dto";

export function useProfileSettings() {
  const navigate = useNavigate();
  const notify = useNotify();
  const queryClient = useQueryClient();

  const [{ data: me }, { data: meta }] = useSuspenseQueries({
    queries: [users.me(), metadata.get()],
  });

  const mutation = useMutation({
    ...users.editMe(),
    onSuccess: async (_, submittedPayload) => {
      await queryClient.invalidateQueries({ queryKey: users.me().queryKey });
      const updatedMe = queryClient.getQueryData<typeof me>(users.me().queryKey);

      const normalizedTimezone = updatedMe?.timezone === "--None--" ? "" : (updatedMe?.timezone ?? "");
      const normalizedPhone = updatedMe?.phoneNumber ?? "";

      const phoneMatches = !submittedPayload.phoneNumber || normalizedPhone === submittedPayload.phoneNumber;
      const timezoneMatches = !submittedPayload.timeZone || normalizedTimezone === submittedPayload.timeZone;

      if (phoneMatches && timezoneMatches) navigate("/profile");
      else notify.error("Failed to update profile. Please try again.");
    },
    onError: () => notify.error("Failed to update profile. Please try again."),
  });

  const buildPayload = (phoneNumber: string, timeZone: string): Partial<EditMeDto> => {
    const payload: Partial<EditMeDto> = {};
    if (phoneNumber !== me.phoneNumber) payload.phoneNumber = phoneNumber;
    if (timeZone !== me.timezone) payload.timeZone = timeZone;
    return payload;
  };

  return {
    me,
    timeZones: meta.timeZones,
    mutation,
    buildPayload,
  };
}
