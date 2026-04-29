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

import { useEffect, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { users } from "@features/users/api/users.queries";
import { projects } from "@features/projects/api/projects.queries";
import { useProject } from "@context/project";
import { useNotify } from "@context/snackbar";
import { useMe } from "@context/me";
import { metadata } from "@features/metadata/api/metadata.queries";
import { getVersion } from "@bridge/index";

export function useProfileData() {
  const notify = useNotify();
  const queryClient = useQueryClient();
  const { projectId, noveraEnabled, kbReferencesEnabled } = useProject();
  const { isAdmin } = useMe();

  const { data } = useQuery(users.me());
  const name = data ? data.firstName + " " + data.lastName : undefined;

  const [isNoveraEnabled, setIsNoveraEnabled] = useState(noveraEnabled);
  const [isKbReferencesEnabled, setIsKbReferencesEnabled] = useState(kbReferencesEnabled);
  const [version, setVersion] = useState<string | undefined>(undefined);

  useEffect(() => { getVersion((v) => setVersion(v)); }, []);

  useEffect(() => { queryClient.prefetchQuery(metadata.get()); }, []);

  const projectEditMutation = useMutation({
    ...projects.edit(projectId!),
    onError: (_, variables) => {
      notify.error("Failed to update project. Please try again.");
      if (variables.hasAgent !== undefined) setIsNoveraEnabled(!variables.hasAgent);
      if (variables.hasKbReferences !== undefined) setIsKbReferencesEnabled(!variables.hasKbReferences);
    },
    onSuccess: () => queryClient.refetchQueries({ queryKey: projects.get(projectId!).queryKey }),
  });

  return {
    data,
    name,
    isAdmin,
    isNoveraEnabled,
    setIsNoveraEnabled,
    isKbReferencesEnabled,
    setIsKbReferencesEnabled,
    version,
    projectEditMutation,
  };
}
