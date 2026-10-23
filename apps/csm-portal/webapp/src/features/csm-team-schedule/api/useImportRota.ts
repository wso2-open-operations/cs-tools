/**
 * Copyright (c) 2026 WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

import { useMutation, useQueryClient, type UseMutationResult } from "@tanstack/react-query";
import { useBackendApi } from "@api/backend/client";

/** The rota import file, schema version 1; sent to the server as it was read. */
export interface RotaImportFile {
  schemaVersion: number;
  family: string;
  rotaCode: string;
  period: { from: string; to: string };
  mode: "replace" | "merge";
  assignments?: unknown[];
  absences?: unknown[];
}

export interface ImportRotaPayload {
  rotaCode: string;
  file: RotaImportFile;
  dryRun: boolean;
}

export interface ImportRotaResult {
  rotaCode: string;
  from: string;
  to: string;
  mode: "replace" | "merge";
  dryRun: boolean;
  cleared: number;
  applied: number;
  skipped: number;
  skippedDays: { index: number; dates: string[] }[];
  absencesCreated: number;
  absencesRemoved: number;
  absencesTrimmed: number;
  /** Earlier time away a replace cut under the file's assignments; absent from an older server. */
  leaveWrittenOver?: number;
}

/** Previews (dryRun) or writes a whole rota for a period from one import file; who may is the server's decision. */
export function useImportRota(): UseMutationResult<ImportRotaResult, Error, ImportRotaPayload> {
  const api = useBackendApi();
  const qc = useQueryClient();
  return useMutation<ImportRotaResult, Error, ImportRotaPayload>({
    mutationFn: ({ rotaCode, file, dryRun }) =>
      api.post<RotaImportFile, ImportRotaResult>(
        `/team-schedule/rotas/${encodeURIComponent(rotaCode)}/import?dryRun=${dryRun}`,
        file,
      ),
    onSuccess: (result) => {
      if (result.dryRun) return;
      void qc.invalidateQueries({ queryKey: ["team-schedule", "assignments"] });
      void qc.invalidateQueries({ queryKey: ["team-schedule", "absences"] });
    },
  });
}
