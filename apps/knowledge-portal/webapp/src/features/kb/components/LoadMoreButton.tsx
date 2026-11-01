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

import { Button, Stack } from "@wso2/oxygen-ui";
import type { JSX } from "react";

interface LoadMoreButtonProps {
  onClick: () => void;
  loading: boolean;
}

/**
 * "Load more" for a paged list. While loading it is `aria-disabled` rather than
 * `disabled`: a disabled button drops keyboard focus to the page body, so the
 * user would have to tab back through everything. The click is ignored instead.
 */
export default function LoadMoreButton({ onClick, loading }: LoadMoreButtonProps): JSX.Element {
  return (
    <Stack alignItems="center" sx={{ mt: 3 }}>
      <Button
        variant="outlined"
        onClick={loading ? undefined : onClick}
        aria-disabled={loading || undefined}
        sx={loading ? { opacity: 0.6, cursor: "progress" } : undefined}
      >
        {loading ? "Loading..." : "Load more"}
      </Button>
    </Stack>
  );
}
