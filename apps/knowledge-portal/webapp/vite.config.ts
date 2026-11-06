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


import { execFileSync } from "node:child_process";
import { fileURLToPath, URL } from "node:url";
import { defineConfig, mergeConfig, type Plugin } from "vite";
import { defineConfig as defineVitestConfig } from "vitest/config";
import react from "@vitejs/plugin-react";

// /config.js is injected per deployment under a fixed filename, so a CDN that
// caches it by bare path can serve a stale copy after a deploy. Tagging the
// <script> src with a build-time version query string forces a fresh fetch per
// build without changing the path the deployment matches against.
function configVersionPlugin(): Plugin {
  let version: string;
  try {
    version = execFileSync("git", ["rev-parse", "--short", "HEAD"], {
      cwd: fileURLToPath(new URL(".", import.meta.url)),
      stdio: ["ignore", "pipe", "ignore"],
    })
      .toString()
      .trim();
  } catch {
    // No .git available (e.g. a Docker build context): fall back to a build
    // timestamp so the query string still changes per build.
    version = String(Date.now());
  }
  return {
    name: "kb-config-version",
    transformIndexHtml(html) {
      return html.replace(
        '<script src="/config.js"></script>',
        `<script src="/config.js?v=${version}"></script>`,
      );
    },
  };
}

const viteConfig = defineConfig({
  plugins: [
    react({
      babel: {
        plugins: [["babel-plugin-react-compiler"]],
      },
    }),
    configVersionPlugin(),
  ],
  resolve: {
    alias: {
      "@": fileURLToPath(new URL("./src", import.meta.url)),
      "@api": fileURLToPath(new URL("./src/api", import.meta.url)),
      "@components": fileURLToPath(
        new URL("./src/components", import.meta.url),
      ),
      "@config": fileURLToPath(new URL("./src/config", import.meta.url)),
      "@features": fileURLToPath(new URL("./src/features", import.meta.url)),
      "@hooks": fileURLToPath(new URL("./src/hooks", import.meta.url)),
      "@layouts": fileURLToPath(new URL("./src/layouts", import.meta.url)),
      "@utils": fileURLToPath(new URL("./src/utils", import.meta.url)),
    },
  },
  server: {
    port: 3100,
    strictPort: true,
  },
});

const vitestConfig = defineVitestConfig({
  test: {
    globals: true,
    environment: "jsdom",
    css: true,
    server: {
      deps: {
        inline: ["@wso2/oxygen-ui", "@wso2/oxygen-ui-icons-react", "@mui/x-data-grid"],
      },
    },
  },
});

export default mergeConfig(viteConfig, vitestConfig);
