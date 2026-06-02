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

import { test as setup } from "@playwright/test";
import { existsSync } from "node:fs";
import { resolve } from "node:path";

/**
 * Pre-flight check for the authenticated project. We do not drive the OIDC
 * sign-in here — see tests/e2e/README.md for the one-time capture flow.
 * This setup exists so the authenticated project fails fast with a clear
 * message instead of hitting the IdP redirect mid-test.
 */
setup("auth storage state must exist", async () => {
  const path = resolve(__dirname, ".auth/user.json");
  if (!existsSync(path)) {
    throw new Error(
      [
        "Authenticated Playwright suite requires a recorded storage state at",
        `  ${path}`,
        "",
        "Run once:",
        "  npx playwright codegen --output=/dev/null \\",
        "    --save-storage=tests/e2e/.auth/user.json \\",
        "    http://localhost:3001/",
        "",
        "Sign in to the IdP in the opened browser, then close it.",
        "See tests/e2e/README.md for details.",
      ].join("\n"),
    );
  }
});
