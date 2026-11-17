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
import { Link2, Printer } from "@wso2/oxygen-ui-icons-react";
import type { JSX } from "react";

interface ArticleActionsProps {
  /** Called with a short message to announce after copying. */
  onNotify: (message: string) => void;
}

/** Copy-link and print controls for an article. */
export default function ArticleActions({ onNotify }: ArticleActionsProps): JSX.Element {
  const copyLink = async (): Promise<void> => {
    try {
      await navigator.clipboard.writeText(window.location.href);
      onNotify("Link copied to clipboard");
    } catch {
      onNotify("The link could not be copied");
    }
  };
  return (
    <Stack direction="row" spacing={1} className="kb-no-print" flexWrap="wrap" useFlexGap>
      <Button
        size="small"
        variant="outlined"
        color="inherit"
        startIcon={<Link2 size={16} aria-hidden />}
        onClick={() => void copyLink()}
      >
        Copy link
      </Button>
      <Button
        size="small"
        variant="outlined"
        color="inherit"
        startIcon={<Printer size={16} aria-hidden />}
        onClick={() => window.print()}
      >
        Print
      </Button>
    </Stack>
  );
}
