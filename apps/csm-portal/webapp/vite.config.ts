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

import { readFileSync } from "node:fs";
import { fileURLToPath, URL } from "node:url";
import { defineConfig, mergeConfig, type Plugin } from "vite";
import { defineConfig as defineVitestConfig } from "vitest/config";
import react from "@vitejs/plugin-react";

const viteConfig = defineConfig({
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
      "@constants": fileURLToPath(new URL("./src/constants", import.meta.url)),
      "@components": fileURLToPath(
        new URL("./src/components", import.meta.url),
      ),
      "@update-cards": fileURLToPath(
        new URL(
          "./src/features/updates/components/update-cards",
          import.meta.url,
        ),
      ),
      "@case-details": fileURLToPath(
        new URL(
          "./src/features/support/components/case-details/header",
          import.meta.url,
        ),
      ),
      "@case-details-attachments": fileURLToPath(
        new URL(
          "./src/features/support/components/case-details/attachments-tab",
          import.meta.url,
        ),
      ),
      "@case-details-details": fileURLToPath(
        new URL(
          "./src/features/support/components/case-details/details-tab",
          import.meta.url,
        ),
      ),
      "@case-details-activity": fileURLToPath(
        new URL(
          "./src/features/support/components/case-details/activity-tab",
          import.meta.url,
        ),
      ),
      "@case-details-calls": fileURLToPath(
        new URL(
          "./src/features/support/components/case-details/calls-tab",
          import.meta.url,
        ),
      ),
      "@config": fileURLToPath(new URL("./src/config", import.meta.url)),
      "@context": fileURLToPath(new URL("./src/context", import.meta.url)),
      "@time-tracking": fileURLToPath(
        new URL(
          "./src/features/project-details/components/time-tracking",
          import.meta.url,
        ),
      ),
      "@deployments": fileURLToPath(
        new URL(
          "./src/features/project-details/components/deployments",
          import.meta.url,
        ),
      ),
      "@hooks": fileURLToPath(new URL("./src/hooks", import.meta.url)),
      "@layouts": fileURLToPath(new URL("./src/layouts", import.meta.url)),
      "@features": fileURLToPath(new URL("./src/features", import.meta.url)),
      "@providers": fileURLToPath(new URL("./src/providers", import.meta.url)),
      "@utils": fileURLToPath(new URL("./src/utils", import.meta.url)),
      buffer: "buffer",
      "buffer/": "buffer/index.js",
    },
  },
  envPrefix: ["CSM_PORTAL_"],
  server: {
    port: 3000,
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

// ── Local full-stack gateway mode (CSM_PORTAL_LOCAL_BE=1, dev server only) ──
// Makes the Vite dev server play the role of the production gateway + runtime
// config for a fully local stack (mock OIDC provider on :9100, csm backend on
// :8080, entity-service on :8081). The app code runs UNMODIFIED: the real OIDC
// SDK performs a genuine authorization-code + PKCE flow against the local IdP.
//
// Two pieces:
//   1. /config.js is served with overrides appended: auth base URL → the
//      local IdP, backend base URL → same-origin /local-api, mocks off.
//   2. /local-api/* is proxied to the local backend with the incoming
//      `Authorization: Bearer <jwt>` mapped to `x-jwt-assertion` — exactly
//      what the production gateway does after validating the token.
//
// Injected ONLY for the dev `serve` command under the env flag: `vite build`
// runs with command === "build", and `vite preview` serves prebuilt dist/, so
// no production artifact can carry this behaviour.

const LOCAL_IDP = "http://localhost:9100";
const LOCAL_BACKEND = "http://127.0.0.1:8080";

const CONFIG_OVERRIDES = `
// --- appended by vite local-gateway mode (CSM_PORTAL_LOCAL_BE=1) ---
window.config.CSM_PORTAL_AUTH_BASE_URL = "${LOCAL_IDP}";
window.config.CSM_PORTAL_AUTH_CLIENT_ID = "csm-local-dev";
// Same-origin backend: derive from the actual origin the dev server is reached
// on, so a forwarded host/IP (devcontainer port-forward, LAN address) still
// hits the Vite /local-api proxy instead of a hardcoded localhost that would
// break the same-origin contract.
window.config.CSM_PORTAL_BACKEND_BASE_URL = window.location.origin + "/local-api";
// Sign-in/out redirects back to the dev server's OWN origin. The base config
// pins these to a fixed host:port (e.g. http://localhost:3000); left unchanged,
// the mock-IdP PKCE callback redirects to that port instead of the port the dev
// server actually runs on, so the SPA never completes the code exchange and
// loops on sign-in. Same same-origin rationale as the backend URL above.
window.config.CSM_PORTAL_AUTH_SIGN_IN_REDIRECT_URL = window.location.origin;
window.config.CSM_PORTAL_AUTH_SIGN_OUT_REDIRECT_URL = window.location.origin;
window.config.CSM_PORTAL_USE_MOCKS = false;
// Pin the runtime mock toggle off so a stale localStorage flag cannot
// silently re-enable mock data in front of the real local backend.
try { localStorage.setItem("csm.useMocks", "0"); } catch (e) { /* ignore */ }
`;

function localGatewayConfigPlugin(): Plugin {
  return {
    name: "csm-local-gateway-config",
    configureServer(server) {
      server.middlewares.use((req, res, next) => {
        if (req.url?.split("?")[0] !== "/config.js") return next();
        let original: string;
        try {
          original = readFileSync(
            fileURLToPath(new URL("./public/config.js", import.meta.url)),
            "utf8",
          );
        } catch (err) {
          server.config.logger.error(
            `[vite] local-gateway: cannot read public/config.js: ${String(err)}`,
          );
          res.statusCode = 500;
          res.setHeader("Content-Type", "text/plain; charset=utf-8");
          res.end("Missing or unreadable public/config.js");
          return;
        }
        res.setHeader("Content-Type", "text/javascript");
        res.setHeader("Cache-Control", "no-store");
        res.end(original + CONFIG_OVERRIDES);
      });
    },
  };
}

export default defineConfig(({ command }) => {
  const merged = mergeConfig(viteConfig, vitestConfig);

  if (command === "serve" && process.env.CSM_PORTAL_LOCAL_BE === "1") {
    console.log(
      "[vite] CSM_PORTAL_LOCAL_BE=1 — serving config overrides (local IdP " +
        `${LOCAL_IDP}, mocks off) and proxying /local-api → ${LOCAL_BACKEND}`,
    );
    return mergeConfig(merged, {
      plugins: [localGatewayConfigPlugin()],
      server: {
        proxy: {
          "/local-api": {
            target: LOCAL_BACKEND,
            changeOrigin: true,
            rewrite: (p: string) => p.replace(/^\/local-api/, ""),
            configure: (proxy: {
              on: (ev: string, cb: (...args: unknown[]) => void) => void;
            }) => {
              proxy.on("proxyReq", (...args: unknown[]) => {
                const proxyReq = args[0] as {
                  setHeader: (k: string, v: string) => void;
                };
                const req = args[1] as {
                  headers: Record<string, string | undefined>;
                };
                const auth = req.headers["authorization"];
                if (auth?.startsWith("Bearer ")) {
                  proxyReq.setHeader("x-jwt-assertion", auth.slice(7));
                }
              });
            },
          },
        },
      },
    });
  }
  return merged;
});
