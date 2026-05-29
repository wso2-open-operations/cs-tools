# 07 — Build System & Microapp Packaging

---

## Important distinction

| Artifact | What it is | Stack |
|----------|-----------|-------|
| `apps/customer-portal/webapp/` | **Public Customer Portal SPA** (this app) | React 19, Vite 7, `base: "/"` |
| `apps/customer-portal/microapp/` | WSO2 Super App embed | Older Oxygen 0.2.x, axios, **`base: "./"`** (relative paths) |

These are **completely separate packages**. The "microapp" packaging request likely refers to
packaging `webapp/dist/` as a deployable zip. They do NOT share a build pipeline.

---

## Build pipeline

```
pnpm run build
  └── tsc -b               # strict typecheck: tsconfig.app.json (src/) + tsconfig.node.json (vite.config.ts)
  └── vite build           # bundle to dist/
        ├── Copies public/ → dist/ verbatim (config.js, logos, example)
        ├── Bundles src/ → dist/assets/
        │   ├── index-<hash>.js         (~4.1 MB main bundle)
        │   ├── html2canvas.esm-<hash>.js (split automatically)
        │   └── index.es-<hash>.js
        └── Rewrites index.html (module script → /assets/index-<hash>.js)
```

**Vite config notes:**
- No `build.manualChunks` — automatic code splitting only
- No `build.minify` override — Vite default (esbuild)
- `base` not set → defaults to `/` (absolute asset URLs)
- `envPrefix: ["CUSTOMER_PORTAL_"]` set but unused by app code

---

## `dist/` output structure

```
dist/
├── index.html              ← serves the SPA; all routes resolve here
├── config.js               ← runtime config (IF present in public/ at build time)
├── config.js.example       ← always present; documentation only
├── logo-dark.svg           ← brand logo (referenced as /logo-dark.svg)
├── logo-white.svg          ← brand logo
└── assets/
    ├── index-<hash>.js     ← main React bundle (~4.1 MB, ~1.4 MB gzipped)
    ├── html2canvas.esm-<hash>.js
    ├── index.es-<hash>.js
    ├── error-401-<hash>.svg
    ├── error-403-<hash>.svg
    ├── error-404-<hash>.svg
    ├── error-500-<hash>.svg
    ├── portal-access-required-<hash>.svg
    ├── project-suspended-<hash>.svg
    └── searching-<hash>.svg
```

`index.html` contains:
```html
<script src="/config.js"></script>           ← root-absolute; must exist at serving root
<link rel="icon" href="/logo-white.svg" />   ← root-absolute
<script type="module" crossorigin src="/assets/index-<hash>.js"></script>
```

---

## Runtime config injection (`config.js` pattern)

The app has **zero environment-specific values baked into the bundle**. Everything is injected at
runtime through `window.config`:

```
Browser loads index.html
  → browser fetches /config.js (served from web root)
  → window.config = { CUSTOMER_PORTAL_AUTH_BASE_URL: "...", CUSTOMER_PORTAL_BACKEND_BASE_URL: "...", ... }
  → React bundle starts → src/config/authConfig.ts reads window.config.CUSTOMER_PORTAL_AUTH_BASE_URL
  → src/config/apiConfig.ts reads window.config.CUSTOMER_PORTAL_BACKEND_BASE_URL
  → ... etc.
```

**Same `dist/` artifact can be promoted across environments by replacing only `config.js`.**

### Minimum required `config.js` keys

```javascript
window.config = {
  CUSTOMER_PORTAL_AUTH_BASE_URL: "https://api.asgardeo.io/t/<tenant>",
  CUSTOMER_PORTAL_AUTH_CLIENT_ID: "<client-id>",
  CUSTOMER_PORTAL_AUTH_SIGN_IN_REDIRECT_URL: "https://<portal-domain>",
  CUSTOMER_PORTAL_AUTH_SIGN_OUT_REDIRECT_URL: "https://<portal-domain>",
  CUSTOMER_PORTAL_BACKEND_BASE_URL: "https://<backend-host>/v1.0",
};
```

All other keys are optional (themes, banners, chatbot, mobile app, log level, mock flags).
See `public/config.js.example` for the complete template.

---

## Packaging as a deployable zip

### Step-by-step

```bash
# 1. Install deps (first time or after package changes)
pnpm install

# 2. Build
pnpm run build

# 3. Create environment-specific config.js
#    IMPORTANT: do NOT use the developer's local config.js
cp public/config.js.example dist/config.js
# Edit dist/config.js with production values for the target environment

# 4. Zip the dist/ directory
cd dist
zip -r ../customer-portal-webapp.zip .
cd ..
```

### What the zip should contain
```
(root of zip)
├── index.html
├── config.js              ← environment-specific (production values)
├── config.js.example      ← optional, documentation
├── logo-dark.svg
├── logo-white.svg
└── assets/
    └── (all hashed JS and SVG files)
```

### What to EXCLUDE from the zip
- `src/` (compiled; not needed at runtime)
- `node_modules/` (not needed at runtime)
- `public/config.js` if it contains local dev values (replace before zipping)
- Tests, tooling configs, README, `.agents/`

---

## Deployment requirements

### Web server configuration
The app uses `BrowserRouter` (HTML5 history API). The web server must serve `index.html` for ALL routes:

**nginx:**
```nginx
server {
  root /var/www/customer-portal;
  index index.html;
  
  location / {
    try_files $uri $uri/ /index.html;
  }
}
```

**Apache:**
```apache
FallbackResource /index.html
```

### Static file serving
- `config.js` must be served from the **root** (`/config.js`) — not in `/assets/`
- `logo-*.svg` must be at root (`/logo-dark.svg`, `/logo-white.svg`)
- `assets/` directory must be publicly accessible with correct MIME types

### Asgardeo / IdP configuration
The IdP application must have these URLs configured:
- **Allowed Callback URLs:** matches `CUSTOMER_PORTAL_AUTH_SIGN_IN_REDIRECT_URL`
- **Allowed Logout URLs:** matches `CUSTOMER_PORTAL_AUTH_SIGN_OUT_REDIRECT_URL`

If using Choreo: configure as SPA; OAuth2/OIDC with PKCE.

---

## Subdirectory / subpath deployment

**Not supported out of the box.** The app uses absolute paths (`/config.js`, `/logo-*.svg`,
`/assets/index-*.js`) and `BrowserRouter` has no `basename`.

To deploy at e.g. `https://host/customer-portal/`:

1. **Vite:** Add `base: '/customer-portal/'` in `vite.config.ts`
2. **Router:** Add `basename="/customer-portal"` to `BrowserRouter` in `AppWithConfig.tsx`
3. **Logo paths:** Update logo `src` in `Header` component (or adjust to use relative paths)
4. **`config.js` script tag:** Update `index.html` `<script src="/customer-portal/config.js">`
5. **IdP redirect URLs:** Update to include subpath

This is NOT currently implemented — changes are required in at least `vite.config.ts`, `AppWithConfig.tsx`, and `index.html`.

---

## Vite configuration summary

```typescript
// vite.config.ts
export default mergeConfig(
  defineConfig({
    plugins: [react({ babel: { plugins: [["babel-plugin-react-compiler", {}]] } })],
    server: { port: 3000 },
    resolve: {
      alias: { /* all path aliases */ }
    },
    envPrefix: ["CUSTOMER_PORTAL_"],
    // No: base, build.manualChunks, build.target, proxy
  }),
  defineConfig({
    test: {
      globals: true,
      environment: "jsdom",
      setupFiles: ["./src/vitest.setup.ts"],
      css: true,
      server: { deps: { inline: ["@wso2/oxygen-ui", "@mui/x-data-grid", ...] } }
    }
  })
);
```

---

## TypeScript build

```
tsconfig.json (solution)
├── tsconfig.app.json     target: ES2022, strict: true, noEmit: true, src/
└── tsconfig.node.json    target: ES2023, strict: true, vite.config.ts only
```

`pnpm run build` runs `tsc -b` first — **build fails on any type error**.
Tests have their own type-checking through Vitest (path aliases match via `vite.config.ts` inheritance).

---

## Environment variables summary

| Mechanism | Used? | Where |
|-----------|-------|-------|
| `window.config` | **Yes** — primary | `public/config.js` → `src/config/*.ts` |
| `import.meta.env` | No | `envPrefix` set but no usage in src/ |
| `.env` files | No | Gitignored; unused |
| Mock toggles (`CUSTOMER_PORTAL_*_USE_MOCK`) | Optional | `window.config` → `portalConfig.ts` |

---

## Vitest (unit tests)

```bash
pnpm test --run       # single run (for CI)
pnpm test             # watch mode
```

- Environment: `jsdom`
- Setup: `src/vitest.setup.ts` (global mocks for Asgardeo, authFetch, DOMPurify, etc.)
- Coverage: NOT configured
- Tests colocated in `__tests__/` next to source files

### `src/vitest.setup.ts` global mocks
- `@asgardeo/react` — `useAsgardeo` returns `{ isSignedIn: true, isLoading: false, getIdToken: () => "mock-token" }`
- `useAuthApiClient` — returns a `vi.fn()` mock
- `dompurify` — `sanitize: (html) => html`, `addHook: vi.fn()`
- `useResolvedInlineImageHtml` — returns `{ resolvedHtml: "", isLoading: false }`
- `useDarkMode` — returns `false`

---

## Known gaps / caveats

| Gap | Detail |
|-----|--------|
| E2E tests | `playwright.config.ts` exists; `tests/e2e/` missing → `pnpm run test:e2e` finds nothing |
| Subpath deploy | Absolute `/` paths; no `basename`/`base` configured |
| `buffer` polyfill | Aliased in `vite.config.ts` for Asgardeo but not in `package.json` dependencies |
| `config.js` in dist | If developer's local `public/config.js` exists, it ends up in `dist/` — CI should inject per environment |
| `ApiQueryKeys.DASHBOARD_STATS` | Defined in constants but unused (dashboard uses separate stat config array) |
| `ApiMutationKeys.POST_CHANGE_REQUEST_COMMENT` | Defined but no hook uses it yet |
| No code splitting | All routes statically imported — main bundle is ~4.1 MB |
| `issueIds` backend support | `CaseSearchFilters.issueIds: number[]` added for category multi-select but backend contract unconfirmed |
