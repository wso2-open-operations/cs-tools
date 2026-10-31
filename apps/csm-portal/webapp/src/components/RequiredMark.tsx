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
import type { JSX } from "react";

/**
 * The required-field asterisk, for a field whose caption the page draws itself
 * (a rich-text field, a radio or checkbox group, a list of rows) and so cannot
 * use MUI's own `required` prop. It is what MUI's FormLabel draws for a required
 * field: a thin space and an asterisk, hidden from assistive tech, in the
 * `MuiFormLabel-asterisk` class so it takes the same colour (including the
 * error colour) wherever the theme styles it. The control itself must still say
 * it is required (`aria-required`, or `required` on the input).
 */
export default function RequiredMark(): JSX.Element {
  return (
    <Box component="span" aria-hidden="true" className="MuiFormLabel-asterisk">
      {" *"}
    </Box>
  );
}
