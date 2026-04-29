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

import { useLayoutEffect } from "react";
import { Skeleton } from "@wso2/oxygen-ui";
import { useLayout } from "@context/layout";

export function usePaginationSubtitleOverride(count?: number | null, total?: number | null) {
  const layout = useLayout();
  const disabled = count === null && total === null;
  const data = count !== undefined && total !== undefined;

  useLayoutEffect(() => {
    if (!disabled)
      layout.setLayoutOverrides({
        subtitle: data ? `${count} of ${total}` : <Skeleton variant="text" width={50} height={20} />,
      });

    return () => layout.setLayoutOverrides({ subtitle: null });
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [count, total]);
}
