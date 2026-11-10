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

import { Alert } from "@wso2/oxygen-ui";
import type { JSX } from "react";

/**
 * A thin note above a page an admin can open only because it is in Preview (set
 * in Settings > Feature visibility), so "I can see this" is never mistaken for
 * "this is live for everyone". Everyone else is sent away and never sees this.
 */
export default function PreviewPageNotice(): JSX.Element {
  return (
    <Alert severity="info" variant="outlined" sx={{ mb: 2 }}>
      <strong>Preview.</strong> This page isn&apos;t open to everyone yet.
    </Alert>
  );
}
