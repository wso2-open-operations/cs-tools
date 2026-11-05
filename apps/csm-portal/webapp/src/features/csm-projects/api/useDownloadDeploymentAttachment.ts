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

import { useCallback } from "react";
import { useBackendApi } from "@api/backend/client";
import { downloadEntityAttachment } from "@features/csm-cases/api/entityAttachments";
import type { DeploymentAttachment } from "@features/csm-projects/types/csmProjects";

/**
 * Returns a function that downloads a deployment attachment: it is streamed
 * through the portal backend (which checks access) and saved under its own name.
 */
export function useDownloadDeploymentAttachment(): (
  attachment: DeploymentAttachment,
) => Promise<void> {
  const api = useBackendApi();

  return useCallback(
    async (attachment: DeploymentAttachment): Promise<void> => {
      await downloadEntityAttachment(api, { type: "deployment", id: attachment.id });
    },
    [api],
  );
}
