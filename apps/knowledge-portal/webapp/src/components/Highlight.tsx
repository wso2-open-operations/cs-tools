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

import { Box } from "@wso2/oxygen-ui";
import { Fragment, type JSX } from "react";
import { primaryTint } from "@config/colorUtils";
import { splitByTerms } from "@utils/highlight";

interface HighlightProps {
  text: string;
  /** Search terms to emphasise. Nothing is marked when empty. */
  query?: string | null;
}

/** `text` with every occurrence of the query's terms wrapped in a `<mark>`. */
export default function Highlight({ text, query }: HighlightProps): JSX.Element {
  const parts = splitByTerms(text, query);
  return (
    <>
      {parts.map((part, index) =>
        part.match ? (
          <Box
            key={index}
            component="mark"
            sx={(theme) => ({
              color: "inherit",
              fontWeight: 600,
              borderRadius: 0.5,
              px: 0.25,
              mx: -0.25,
              bgcolor: primaryTint(theme, 0.22),
            })}
          >
            {part.text}
          </Box>
        ) : (
          <Fragment key={index}>{part.text}</Fragment>
        ),
      )}
    </>
  );
}
