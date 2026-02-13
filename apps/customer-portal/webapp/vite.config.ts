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

import { fileURLToPath, URL } from "node:url";
import { defineConfig } from "vitest/config";
import react from "@vitejs/plugin-react";

export default defineConfig({
  plugins: [
    react({
      babel: {
        plugins: [["babel-plugin-react-compiler"]],
      },
    }),
  ],
  resolve: {
    alias: {
      "@": fileURLToPath(new URL("./src", import.meta.url)),
      "@api": fileURLToPath(new URL("./src/api", import.meta.url)),
      "@assets": fileURLToPath(new URL("./src/assets", import.meta.url)),
      "@components": fileURLToPath(
        new URL("./src/components", import.meta.url),
      ),
      "@case-details": fileURLToPath(
        new URL("./src/components/support/case-details/header", import.meta.url),
      ),
      "@case-details-attachments": fileURLToPath(
        new URL("./src/components/support/case-details/attachments-tab", import.meta.url),
      ),
      "@case-details-details": fileURLToPath(
        new URL("./src/components/support/case-details/details-tab", import.meta.url),
      ),
      "@config": fileURLToPath(new URL("./src/config", import.meta.url)),
      "@constants": fileURLToPath(new URL("./src/constants", import.meta.url)),
      "@context": fileURLToPath(new URL("./src/context", import.meta.url)),
      "@hooks": fileURLToPath(new URL("./src/hooks", import.meta.url)),
      "@layouts": fileURLToPath(new URL("./src/layouts", import.meta.url)),
      "@models": fileURLToPath(new URL("./src/models", import.meta.url)),
      "@pages": fileURLToPath(new URL("./src/pages", import.meta.url)),
      "@providers": fileURLToPath(new URL("./src/providers", import.meta.url)),
      "@utils": fileURLToPath(new URL("./src/utils", import.meta.url)),
      buffer: "buffer",
      "buffer/": "buffer/index.js",
    },
  },
  envPrefix: ["CUSTOMER_PORTAL_"],
  server: {
    port: 3000,
  },
  test: {
    globals: true,
    environment: "jsdom",
    setupFiles: ["./src/vitest.setup.ts"],
    css: true,
    server: {
      deps: {
        inline: [
          "@wso2/oxygen-ui",
          "@wso2/oxygen-ui-icons-react",
          "@wso2/oxygen-ui-charts-react",
          "@mui/x-data-grid",
          "@asgardeo/browser",
          "buffer",
        ],
      },
    },
  },
});
