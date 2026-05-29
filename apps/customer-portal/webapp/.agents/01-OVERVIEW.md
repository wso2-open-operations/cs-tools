# 01 — Overview: Stack, Structure & Conventions

## Tech stack

| Concern | Library / tool | Version |
|---------|----------------|---------|
| Framework | React | 19.2.0 |
| Build tool | Vite | 7.2.4 |
| TypeScript | typescript | 5.9.3 |
| Router | react-router | 7.1.5 |
| Server state | @tanstack/react-query | 5.90.20 |
| Auth | @asgardeo/react | 0.22.4 |
| UI library | @wso2/oxygen-ui | 0.6.0 |
| UI icons | @wso2/oxygen-ui-icons-react | 0.6.0 |
| UI charts | @wso2/oxygen-ui-charts-react | 0.6.0 |
| Rich text | lexical + @lexical/* | 0.40.0 |
| HTML sanitizer | dompurify | 3.3.1 |
| PDF | jspdf + jspdf-autotable | 4.2.0 / 5.0.7 |
| Markdown | markdown-it + react-markdown | ^14.1.1 / 10.1.0 |
| Phone input | react-phone-number-input | 3.4.16 |
| Idle timer | react-idle-timer | 5.7.2 |
| Syntax highlight | prismjs | 1.30.0 |
| Unit tests | vitest | 4.0.18 |
| Test render | @testing-library/react | 16.3.2 |
| E2E | @playwright/test | 1.49.0 (config only, tests missing) |
| Linter | eslint 9 (flat config) | 9.39.1 |
| Package manager | pnpm | 9+ |

## Repository context

```
cs-tools/                       ← monorepo root
└── apps/
    └── customer-portal/
        ├── backend/            ← Ballerina REST API
        ├── webapp/             ← THIS APP (React SPA)
        └── microapp/           ← Separate Super App bundle (different stack, base: "./")
```

`webapp/` is a self-contained package (`package.json` + `pnpm-lock.yaml`). It does NOT share packages
with `microapp/` or `backend/`.

## Folder structure

```
webapp/
├── index.html                  ← Vite HTML entry; loads /config.js first
├── vite.config.ts              ← Vite + Vitest (merged via mergeConfig)
├── tsconfig.json               ← Solution-style: references app + node configs
├── tsconfig.app.json           ← App TypeScript (src/)
├── tsconfig.node.json          ← Tooling TypeScript (vite.config.ts)
├── eslint.config.js            ← ESLint 9 flat config
├── playwright.config.ts        ← E2E (testDir missing)
├── package.json
├── pnpm-lock.yaml
├── public/
│   ├── config.js.example       ← Committed runtime config template
│   ├── config.js               ← GITIGNORED; placed per environment
│   ├── logo-dark.svg
│   └── logo-white.svg
├── dist/                       ← Build output (gitignored)
│   ├── index.html
│   ├── config.js(.example)
│   ├── logo-*.svg
│   └── assets/
│       ├── index-<hash>.js     ← ~4.1 MB main bundle
│       ├── html2canvas.esm-<hash>.js
│       ├── index.es-<hash>.js
│       └── *.svg               ← Hashed error/access-control SVGs
└── src/
    ├── main.tsx                ← React.StrictMode → AppWithConfig
    ├── AppWithConfig.tsx       ← Outer providers + BrowserRouter
    ├── App.tsx                 ← Route tree + inner UI contexts
    ├── vitest.setup.ts         ← Global test mocks
    ├── api/                    ← Shared TanStack Query hooks (cross-feature)
    ├── assets/                 ← Bundled SVGs (imported as modules)
    ├── components/             ← Shared UI components
    ├── config/                 ← window.config readers (authConfig, apiConfig, etc.)
    ├── constants/              ← ApiQueryKeys, route/common/auth constants
    ├── context/                ← React contexts (loader, error banner, logger, etc.)
    ├── features/               ← Domain modules
    │   ├── announcements/
    │   ├── dashboard/
    │   ├── engagements/
    │   ├── operations/
    │   ├── project-details/
    │   ├── project-hub/
    │   ├── security/
    │   ├── settings/
    │   ├── support/
    │   ├── updates/
    │   └── usage-metrics/
    ├── hooks/                  ← Shared hooks
    ├── layouts/                ← Shell layouts and route guards
    ├── providers/              ← Cross-cutting providers
    ├── types/                  ← Shared types
    └── utils/                  ← Shared utilities
```

### Feature module anatomy

Each feature under `src/features/<feature>/` typically contains:

```
<feature>/
├── api/            ← TanStack Query hooks for this feature's endpoints
├── components/     ← UI components (grouped in sub-dirs for large features)
│   └── __tests__/  ← Colocated Vitest tests
├── constants/      ← Feature-specific constants, filter defs, enums
├── hooks/          ← Custom React hooks (rare — most features skip this)
├── pages/          ← Page-level components (composition only, no inline implementations)
│   └── __tests__/
├── types/          ← Domain types (API shapes, component props, enums)
└── utils/          ← Business logic, formatters, request builders
    └── __tests__/
```

## Path aliases

Declared identically in `vite.config.ts` (resolve.alias) and `tsconfig.app.json` (paths).
**Always update both files when adding a new alias.**

| Alias | Resolves to |
|-------|-------------|
| `@` / `@/*` | `src/` |
| `@api` | `src/api` |
| `@assets` | `src/assets` |
| `@components` | `src/components` |
| `@config` | `src/config` |
| `@constants` | `src/constants` |
| `@context` | `src/context` |
| `@hooks` | `src/hooks` |
| `@layouts` | `src/layouts` |
| `@features` | `src/features` |
| `@providers` | `src/providers` |
| `@utils` | `src/utils` |
| `@update-cards` | `src/features/updates/components/update-cards` |
| `@case-details` | `src/features/support/components/case-details/header` |
| `@case-details-attachments` | `src/features/support/components/case-details/attachments-tab` |
| `@case-details-details` | `src/features/support/components/case-details/details-tab` |
| `@case-details-activity` | `src/features/support/components/case-details/activity-tab` |
| `@case-details-calls` | `src/features/support/components/case-details/calls-tab` |
| `@time-tracking` | `src/features/project-details/components/time-tracking` |
| `@deployments` | `src/features/project-details/components/deployments` |
| `buffer` / `buffer/` | buffer polyfill (for Asgardeo) |

**Note:** There is no `@models`, `@pages`, or `@layouts` alias pointing to a unified pages dir.
Pages live under `src/features/<feature>/pages/`.

## Coding conventions (mandatory)

### License header

Every `.ts` / `.tsx` source file must begin with exactly these 15 lines followed by a blank line:

```typescript
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
// KIND, either express or implied. See the License for the
// specific language governing permissions and limitations
// under the License.
```

Reference: `src/layouts/AppLayout.tsx` lines 1–15.

`public/config.js.example` uses a `/** ... */` JSDoc block of the same text.
`eslint.config.js` and `index.html` do NOT carry the header.

### Component comments

- React components: JSDoc with description + `@returns {JSX.Element}`
- Functions/hooks: JSDoc with `@param` and `@returns`
- Inline: `//` comments only for non-obvious intent; never narrate what the code does

### Styling

- **Oxygen UI `sx` prop only** — no CSS files, no styled-components
- Theme: **Acrylic Orange** (`OxygenUIThemeProvider theme={themeConfig}`)
- **No `borderRadius`** except `borderRadius: "50%"` for circular elements
- Use `theme.palette`, `theme.breakpoints`, `alpha()` for colors in `sx`
- Dark mode via `useDarkMode()` which watches `<html data-color-scheme="dark">`

### Pages rule

Page files (`src/features/*/pages/*.tsx`) **must only compose** components from `src/components/`
and the feature's `components/` directory. No inline component implementations in page files.

### Types

- Define TypeScript types/interfaces for all props, API payloads, and response shapes
- Feature props live in `src/features/<feature>/types/<feature>Components.ts`
- API types live in `src/features/<feature>/types/<feature>.ts` or similar

### Tests

- Every created component must have a colocated test in `__tests__/`
- Test framework: Vitest + `@testing-library/react`
- Use path aliases in imports (same as source)
- Global mocks are in `src/vitest.setup.ts`

## `window.config` runtime pattern

Config is NOT baked into the bundle. The browser loads `/config.js` before React starts:

```
index.html
  └── <script src="/config.js"> → sets window.config = { CUSTOMER_PORTAL_*: "..." }
  └── React bundle
        └── src/config/*.ts read window.config at module load
```

- `src/config/portalConfig.ts` — defines the `CustomerPortalWindowConfig` type on `window`
- `src/config/authConfig.ts` — reads auth URLs, throws if missing
- `src/config/apiConfig.ts` — reads `BACKEND_BASE_URL`
- `src/config/themeConfig.ts` — picks theme from `CUSTOMER_PORTAL_THEME`
- `src/config/loggerConfig.ts` — reads `CUSTOMER_PORTAL_LOG_LEVEL`

**No `import.meta.env` usage in app code.** The `envPrefix: ["CUSTOMER_PORTAL_"]` in vite.config.ts
is set but unused in practice.

## Scripts

| Script | Command |
|--------|---------|
| `pnpm run dev` | Dev server on port 3000 |
| `pnpm run build` | `tsc -b && vite build` → `dist/` |
| `pnpm run lint` | `eslint .` |
| `pnpm run preview` | Serve `dist/` locally |
| `pnpm test --run` | Unit tests (single run) |
| `pnpm test` | Unit tests (watch mode) |
| `pnpm run test:e2e` | Playwright (tests missing) |
